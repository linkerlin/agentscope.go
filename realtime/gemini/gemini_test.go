// realtime/gemini/gemini_test.go — local WebSocket mock contract tests (19.6
// acceptance): the Gemini Live backend passes the SAME contract test set as
// the DashScope (19.4) and OpenAI (19.5) backends — handshake, full
// audio+tools+metrics chain through RealtimeAgent, interrupt, control-frame
// shapes, errors, cards — not just client construction. The mock speaks the
// live protocol shapes (setup/setupComplete, realtimeInput, clientContent,
// serverContent, toolCall).
package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
)

var pcm16k = realtime.AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}

// ---- mock Gemini Live endpoint ----

type mockLiveServer struct {
	srv *httptest.Server

	mu            sync.Mutex
	queryKey      string
	authHeader    string
	setups        []setupRequest
	clientMsgs    []string // client frame types in order ("setup" / "realtimeInput" / "clientContent")
	audios        [][]byte
	texts         []string
	emptyTurns    int // clientContent frames with no turns (near-cancel)
	conns         atomic.Int32
	failHandshake bool

	// autoFrames are pushed right after setupComplete on connection n.
	autoFrames func(n int) []string
	// respond returns frames for one client message; nil = none.
	respond func(clientType string, raw []byte) []string
}

func newMockLiveServer(t *testing.T) *mockLiveServer {
	t.Helper()
	m := &mockLiveServer{}
	up := websocket.Upgrader{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		n := int(m.conns.Add(1))
		m.mu.Lock()
		m.queryKey = r.URL.Query().Get("key")
		m.authHeader = r.Header.Get("Authorization")
		m.mu.Unlock()
		write := func(frame string) bool {
			return conn.WriteMessage(websocket.TextMessage, []byte(frame)) == nil
		}
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			clientType, _ := m.record(data)
			var frames []string
			if clientType == "setup" {
				if m.failHandshake {
					write(frameOf(serverFrame{Error: &struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}{Code: 3, Message: "API key not valid"}}))
					return
				}
				frames = append(frames, frameOf(serverFrame{SetupComplete: &struct{}{}}))
				if m.autoFrames != nil {
					frames = append(frames, m.autoFrames(n)...)
				}
			} else if m.respond != nil {
				frames = m.respond(clientType, data)
			}
			for _, fr := range frames {
				if !write(fr) {
					return
				}
			}
		}
	}))
	t.Cleanup(func() { m.srv.Close() })
	return m
}

func (m *mockLiveServer) URL() string { return m.srv.URL }

// record parses one client frame and books it; returns the frame type
// ("setup" / "realtimeInput" / "clientContent" / "other").
func (m *mockLiveServer) record(data []byte) (string, *setupRequest) {
	var f clientFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return "other", nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case f.Setup != nil:
		m.clientMsgs = append(m.clientMsgs, "setup")
		m.setups = append(m.setups, *f.Setup)
		return "setup", f.Setup
	case f.RealtimeInput != nil:
		m.clientMsgs = append(m.clientMsgs, "realtimeInput")
		if raw, err := base64.StdEncoding.DecodeString(f.RealtimeInput.Audio); err == nil {
			m.audios = append(m.audios, raw)
		}
		return "realtimeInput", nil
	case f.ClientContent != nil:
		m.clientMsgs = append(m.clientMsgs, "clientContent")
		if len(f.ClientContent.Turns) == 0 {
			m.emptyTurns++
		}
		for _, t := range f.ClientContent.Turns {
			for _, p := range t.Parts {
				if p.Text != "" {
					m.texts = append(m.texts, p.Text)
				}
			}
		}
		return "clientContent", nil
	default:
		m.clientMsgs = append(m.clientMsgs, "other")
		return "other", nil
	}
}

func (m *mockLiveServer) snapshot() (msgs, texts []string, audios [][]byte, emptyTurns int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.clientMsgs...), append([]string(nil), m.texts...),
		append([][]byte(nil), m.audios...), m.emptyTurns
}

// ---- frame builders ----

func frameOf(e serverFrame) string {
	b, _ := json.Marshal(e)
	return string(b)
}

func serverContentFrame(sc serverContent) string {
	return frameOf(serverFrame{ServerContent: &sc})
}

func modelTextFrame(text string) string {
	sc := serverContent{}
	sc.ModelTurn = &struct{ Parts []contentPart }{Parts: []contentPart{{Text: text}}}
	return serverContentFrame(sc)
}

func modelAudioFrame(data []byte) string {
	sc := serverContent{}
	sc.ModelTurn = &struct{ Parts []contentPart }{Parts: []contentPart{{InlineData: &struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	}{MimeType: outputAudioMime, Data: base64.StdEncoding.EncodeToString(data)}}}}
	return serverContentFrame(sc)
}

func inputTranscriptionFrame(text string) string {
	sc := serverContent{}
	sc.InputTranscription = &struct{ Text string }{Text: text}
	return serverContentFrame(sc)
}

func interruptedFrame() string {
	sc := serverContent{Interrupted: true}
	return serverContentFrame(sc)
}

func turnCompleteFrame() string {
	sc := serverContent{TurnComplete: true}
	return serverContentFrame(sc)
}

func toolCallFrame(name string, args string, idx int) string {
	tc := &struct {
		FunctionCalls []functionCall `json:"functionCalls"`
	}{}
	tc.FunctionCalls = append(tc.FunctionCalls, functionCall{Name: name, Args: json.RawMessage(args)})
	return frameOf(serverFrame{ToolCall: tc})
}

// ---- shared assertions (same shapes as the dashscope/openai suites) ----

func typeName(ev realtime.Event) string {
	switch ev.(type) {
	case realtime.SessionStarted:
		return "started"
	case realtime.InputTranscriptDelta:
		return "input_delta"
	case realtime.UserTranscribed:
		return "user_done"
	case realtime.ResponseStarted:
		return "response_started"
	case realtime.TranscriptDelta:
		return "text"
	case realtime.AudioOutDelta:
		return "audio"
	case realtime.ToolCall:
		return "tool_call"
	case realtime.ToolResult:
		return "tool_result"
	case realtime.Interrupted:
		return "interrupted"
	case realtime.ResponseDone:
		return "response_done"
	case realtime.ErrorEvent:
		return "error"
	case realtime.SessionClosed:
		return "closed"
	default:
		return "unknown"
	}
}

func collect(t *testing.T, out <-chan realtime.Event, pred func(realtime.Event) bool) []string {
	t.Helper()
	var names []string
	for ev := range out {
		if realtime.Terminal(ev) || pred(ev) {
			return append(names, typeName(ev))
		}
		names = append(names, typeName(ev))
	}
	return names
}

func assertSeq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event sequence[%d]: got %s want %s (full: %v)", i, got[i], want[i], got)
		}
	}
}

// ---- tests (the same contract set as 19.4/19.5) ----

// TestConnect_Handshake locks the connect contract: API key on the query,
// the setup frame shape (models/ prefix, AUDIO modality, voice, tools,
// transcription toggles) and the setupComplete wait.
func TestConnect_Handshake(t *testing.T) {
	srv := newMockLiveServer(t)
	model := NewLiveFlash("g-key", WithBaseURL(srv.URL()),
		WithTools(ToolDef{Name: "weather", Description: "query weather", Parameters: json.RawMessage(`{"type":"object"}`)}))

	sess, answer, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if !answer.Format.Equal(pcm16k) {
		t.Fatalf("negotiated: %v", answer.Format)
	}
	if ev := <-sess.Events(); ev == nil {
		t.Fatal("no session started event")
	}

	srv.mu.Lock()
	key := srv.queryKey
	setups := append([]setupRequest(nil), srv.setups...)
	srv.mu.Unlock()
	if key != "g-key" {
		t.Fatalf("api key must ride the query: %q", key)
	}
	if len(setups) != 1 {
		t.Fatalf("setups: %d", len(setups))
	}
	s := setups[0]
	if s.Model != "models/gemini-live-2.5-flash-preview" {
		t.Fatalf("setup model: %q", s.Model)
	}
	if len(s.GenerationConfig.ResponseModalities) != 1 || s.GenerationConfig.ResponseModalities[0] != "AUDIO" {
		t.Fatalf("modalities: %v", s.GenerationConfig.ResponseModalities)
	}
	if s.GenerationConfig.SpeechConfig == nil || s.GenerationConfig.SpeechConfig.VoiceConfig.VoiceName != "Aoora" {
		t.Fatalf("voice config: %+v", s.GenerationConfig.SpeechConfig)
	}
	if len(s.Tools) != 1 || len(s.Tools[0].FunctionDeclarations) != 1 || s.Tools[0].FunctionDeclarations[0].Name != "weather" {
		t.Fatalf("tools: %+v", s.Tools)
	}
	if s.InputAudioTranscription == nil || s.OutputAudioTranscription == nil {
		t.Fatalf("transcription toggles must be on: %+v", s)
	}
}

// TestFullChain_AudioToolsMetrics: one duplex turn (input transcription,
// transcript, two audio parts, one tool call) pushed by the server after the
// handshake, driven through RealtimeAgent with TurnMetrics — the same
// acceptance shape as 19.4/19.5.
func TestFullChain_AudioToolsMetrics(t *testing.T) {
	srv := newMockLiveServer(t)
	srv.autoFrames = func(n int) []string {
		return []string{
			inputTranscriptionFrame("北京天气"),
			modelTextFrame("北京晴"),
			modelAudioFrame([]byte{1, 2, 3}),
			modelAudioFrame([]byte{4, 5, 6}),
			toolCallFrame("weather", `{"city":"北京"}`, 0),
			turnCompleteFrame(),
		}
	}
	model := NewLiveFlash("g-key", WithBaseURL(srv.URL()),
		WithTools(ToolDef{Name: "weather", Description: "query weather"}))

	metricsCh := make(chan realtime.TurnMetrics, 2)
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer:         realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
		OnTurnMetrics: func(m realtime.TurnMetrics) { metricsCh <- m },
	}).WithTools(realtime.ToolHandlerFunc(func(ctx context.Context, call realtime.ToolCall) ([]byte, error) {
		if call.Name != "weather" || string(call.Args) != `{"city":"北京"}` {
			t.Errorf("tool call: %+v", call)
		}
		return []byte(`{"cond":"sunny"}`), nil
	}))

	out, err := agent.Connect(context.Background())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	names := collect(t, out, func(ev realtime.Event) bool {
		rd, ok := ev.(realtime.ResponseDone)
		return ok && rd.Final
	})
	// The synthesized ResponseStarted rides the first modelTurn part.
	assertSeq(t, names, []string{
		"started", "user_done", "response_started", "text", "audio", "audio", "tool_call", "tool_result", "response_done",
	})

	select {
	case m := <-metricsCh:
		if !m.Final || m.AudioChunks != 2 || m.AudioBytes != 6 || m.ToolCalls != 1 {
			t.Fatalf("turn metrics: %+v", m)
		}
		if m.TimeToFirstAudio() < 0 || m.TurnDuration() < m.TimeToFirstAudio() {
			t.Fatalf("turn metrics latencies: %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("OnTurnMetrics did not fire")
	}
	_ = agent.Close()
}

// TestInterrupt_ServerCard: the near-cancel goes on the wire as an EMPTY
// clientContent turn; the server's interrupted + turnComplete close the turn
// as NOT final, and (server card) the local playout keeps its queue.
func TestInterrupt_ServerCard(t *testing.T) {
	srv := newMockLiveServer(t)
	srv.autoFrames = func(n int) []string {
		return []string{
			modelAudioFrame([]byte{7, 7, 7}),
			inputTranscriptionFrame("停"),
		}
	}
	srv.respond = func(clientType string, raw []byte) []string {
		if clientType != "clientContent" {
			return nil
		}
		var f clientFrame
		_ = json.Unmarshal(raw, &f)
		if f.ClientContent != nil && len(f.ClientContent.Turns) == 0 {
			return []string{interruptedFrame(), turnCompleteFrame()}
		}
		return nil
	}
	model := NewLiveFlash("g-key", WithBaseURL(srv.URL()))
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
	})
	out, err := agent.Connect(context.Background())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	names := collect(t, out, func(ev realtime.Event) bool {
		rd, ok := ev.(realtime.ResponseDone)
		return ok && !rd.Final
	})
	assertSeq(t, names, []string{"started", "response_started", "audio", "user_done", "interrupted", "response_done"})

	if _, _, _, empty := srv.snapshot(); empty != 1 {
		t.Fatalf("interrupt must be exactly one empty clientContent turn: %d", empty)
	}
	// Server-truncation card: nothing local is dropped.
	if pending := agent.Playout().Pending(); pending != 3 {
		t.Fatalf("server card must not truncate local playout, pending=%d", pending)
	}
	_ = agent.Close()
}

// TestSendControls locks the control-frame shapes: realtimeInput base64
// audio, clientContent text turns.
func TestSendControls(t *testing.T) {
	srv := newMockLiveServer(t)
	model := NewLiveFlash("g-key", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	chunk := []byte{0x0a, 0x0b, 0x0c}
	if err := sess.SendAudio(context.Background(), chunk); err != nil {
		t.Fatalf("send audio: %v", err)
	}
	if err := sess.SendText(context.Background(), "你好"); err != nil {
		t.Fatalf("send text: %v", err)
	}
	if err := sess.SendAudio(context.Background(), nil); err != nil {
		t.Fatalf("empty audio: %v", err)
	}
	if err := sess.Interrupt(context.Background()); err != nil {
		t.Fatalf("interrupt: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs, texts, audios, empty := srv.snapshot()
		if len(msgs) >= 4 {
			if msgs[1] != "realtimeInput" || msgs[2] != "clientContent" || msgs[3] != "clientContent" {
				t.Fatalf("client frame order: %v", msgs)
			}
			if len(audios) != 1 || string(audios[0]) != "\x0a\x0b\x0c" {
				t.Fatalf("audio round-trip: %v", audios)
			}
			if len(texts) != 1 || texts[0] != "你好" {
				t.Fatalf("texts: %v", texts)
			}
			if empty != 1 {
				t.Fatalf("empty turns: %d", empty)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not receive all client frames in time")
}

// TestError_Terminal: an error message maps to ErrorEvent and ends the
// stream.
func TestError_Terminal(t *testing.T) {
	srv := newMockLiveServer(t)
	srv.autoFrames = func(n int) []string {
		errFrame := serverFrame{Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: 13, Message: "internal"}}
		return []string{frameOf(errFrame)}
	}
	model := NewLiveFlash("g-key", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	for ev := range sess.Events() {
		if _, isStart := ev.(realtime.SessionStarted); isStart {
			continue
		}
		ee, isErr := ev.(realtime.ErrorEvent)
		if !isErr {
			t.Fatalf("expected ErrorEvent, got %T", ev)
		}
		if !strings.Contains(ee.Err, "internal") {
			t.Fatalf("error message: %q", ee.Err)
		}
		if _, ok := <-sess.Events(); ok {
			t.Fatalf("stream must close after ErrorEvent")
		}
		return
	}
	t.Fatalf("stream closed without the error event")
}

// TestHandshake_Error: an error before setupComplete fails Connect.
func TestHandshake_Error(t *testing.T) {
	srv := newMockLiveServer(t)
	srv.failHandshake = true
	model := NewLiveFlash("g-bad", WithBaseURL(srv.URL()))
	_, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err == nil {
		t.Fatalf("connect must fail on handshake error")
	}
	if !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("error must surface the server message: %v", err)
	}
}

// TestDialError_RedactsAPIKey: the endpoint embeds the key in the query —
// a dial failure wrapping a *url.Error must never leak it into the error
// chain (logs), while errors.As on the chain keeps working.
func TestDialError_RedactsAPIKey(t *testing.T) {
	// Pure redaction: a *url.Error carrying the key comes back masked.
	inner := &url.Error{Op: "Get", URL: "wss://example.invalid/x?key=super-secret-key&alt=json", Err: errors.New("boom")}
	red := redactURLKey(inner)
	msg := red.Error()
	if strings.Contains(msg, "super-secret-key") {
		t.Fatalf("redacted error leaks the key: %s", msg)
	}
	if !strings.Contains(msg, "key=REDACTED") {
		t.Fatalf("redaction marker missing: %s", msg)
	}
	var ue *url.Error
	if !errors.As(red, &ue) {
		t.Fatalf("redacted error must stay a *url.Error for errors.As")
	}

	// Integration: a failed dial against a dead port surfaces no key.
	model := NewLiveFlash("super-secret-key", WithBaseURL("http://127.0.0.1:1/dead"))
	_, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err == nil {
		t.Fatalf("dial to a dead port must fail")
	}
	if strings.Contains(err.Error(), "super-secret-key") {
		t.Fatalf("connect error leaks the api key: %s", err.Error())
	}

	// Non-URL errors pass through untouched.
	plain := errors.New("plain")
	if redactURLKey(plain) != plain {
		t.Fatalf("non-url errors must pass through")
	}
}

// TestCards_Embedded: both live cards load with server truncation and
// duplex pcm (16k in / 24k out).
func TestCards_Embedded(t *testing.T) {
	cards, err := ListModelCards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("want 2 cards, got %d", len(cards))
	}
	pcm24k := realtime.AudioFormat{Codec: "pcm", SampleRate: 24000, Channels: 1}
	for _, c := range cards {
		if c.Truncation.Normalized() != realtime.TruncationServer || !c.Tools {
			t.Fatalf("card %s capabilities: %+v", c.ID, c)
		}
		if len(c.AudioIn) != 1 || !c.AudioIn[0].Equal(pcm16k) || len(c.AudioOut) != 1 || !c.AudioOut[0].Equal(pcm24k) {
			t.Fatalf("card %s formats: in=%+v out=%+v", c.ID, c.AudioIn, c.AudioOut)
		}
	}
}
