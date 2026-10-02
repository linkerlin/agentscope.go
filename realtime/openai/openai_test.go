// realtime/openai/openai_test.go — local WebSocket mock contract tests (19.5
// acceptance): text, duplex audio, cancellation and reconnection over the
// same contract test set as 19.4 (mock endpoint + scripted frames + full
// event-sequence assertions through RealtimeAgent). The mock speaks the real
// OpenAI Realtime event vocabulary; the live smoke test is opt-in
// (smoke_test.go).
package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
)

var pcm24k = realtime.AudioFormat{Codec: "pcm", SampleRate: 24000, Channels: 1}

// ---- mock OpenAI Realtime endpoint ----

type mockRTServer struct {
	srv *httptest.Server

	mu             sync.Mutex
	authHeader     string
	betaHeader     string
	queryModel     string
	sessionIDs     []string
	sessionCfgs    []sessionConfig
	clientTypes    []string
	audios         [][]byte
	texts          []string
	cancels        int
	conns          atomic.Int32
	failStart      bool
	closeAfterAuto bool

	// autoFrames returns server frames pushed right after session.updated
	// on connection n (1-based); nil = none.
	autoFrames func(n int) []string
	// respond returns frames for one client event; nil = none.
	respond func(evType string, e clientEvent) []string
}

func newMockRTServer(t *testing.T) *mockRTServer {
	t.Helper()
	m := &mockRTServer{}
	up := websocket.Upgrader{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		n := int(m.conns.Add(1))
		m.mu.Lock()
		m.authHeader = r.Header.Get("Authorization")
		m.betaHeader = r.Header.Get("OpenAI-Beta")
		m.queryModel = r.URL.Query().Get("model")
		m.mu.Unlock()

		write := func(frame string) bool {
			return conn.WriteMessage(websocket.TextMessage, []byte(frame)) == nil
		}
		if m.failStart {
			write(frameOf(serverEvent{Type: evError, Error: &errorObj{Code: "invalid_api_key", Message: "Incorrect API key"}}))
			return
		}
		sessID := "sess_mock_" + strconv.Itoa(n)
		if !write(frameOf(serverEvent{Type: evSessionCreated, Session: &sessionRef{ID: sessID}})) {
			return
		}
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var e clientEvent
			if err := json.Unmarshal(data, &e); err != nil {
				continue
			}
			m.record(e)
			var frames []string
			dropAfterWrite := false
			if e.Type == evSessionUpdate {
				m.mu.Lock()
				if e.Session != nil {
					m.sessionCfgs = append(m.sessionCfgs, *e.Session)
				}
				m.mu.Unlock()
				frames = append(frames, frameOf(serverEvent{Type: evSessionUpdated, Session: &sessionRef{ID: sessID}}))
				if m.autoFrames != nil {
					frames = append(frames, m.autoFrames(n)...)
				}
				dropAfterWrite = m.closeAfterAuto
			} else if m.respond != nil {
				frames = m.respond(e.Type, e)
			}
			for _, fr := range frames {
				if !write(fr) {
					return
				}
			}
			if dropAfterWrite {
				// Drop AFTER flushing the auto frames: the client must
				// consume the scripted turn, then observe the disconnect.
				_ = conn.Close()
				return
			}
		}
	}))
	t.Cleanup(func() { m.srv.Close() })
	return m
}

func (m *mockRTServer) URL() string { return m.srv.URL }

func (m *mockRTServer) record(e clientEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientTypes = append(m.clientTypes, e.Type)
	switch e.Type {
	case evAudioAppend:
		if raw, err := base64.StdEncoding.DecodeString(e.Audio); err == nil {
			m.audios = append(m.audios, raw)
		}
	case evItemCreate:
		if content, ok := e.Item["content"].([]any); ok && len(content) > 0 {
			if c, ok := content[0].(map[string]any); ok {
				if text, ok := c["text"].(string); ok {
					m.texts = append(m.texts, text)
				}
			}
		}
	case evResponseCancel:
		m.cancels++
	}
}

func (m *mockRTServer) snapshot() (types, texts []string, audios [][]byte, cancels int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.clientTypes...), append([]string(nil), m.texts...),
		append([][]byte(nil), m.audios...), m.cancels
}

// ---- frame builders ----

func frameOf(e serverEvent) string {
	b, _ := json.Marshal(e)
	return string(b)
}

func transcriptDone(text string) string {
	return frameOf(serverEvent{Type: evTranscriptDone, Transcript: text})
}

func responseCreated(id string) string {
	return frameOf(serverEvent{Type: evResponseStart, Response: &responseObj{ID: id}})
}

func textDelta(s string) string {
	return frameOf(serverEvent{Type: evTextDelta, Delta: s})
}

func audioDelta(data []byte) string {
	return frameOf(serverEvent{Type: evAudioDelta, Delta: base64.StdEncoding.EncodeToString(data)})
}

func functionCallDone(callID, name, args string) string {
	return frameOf(serverEvent{Type: evItemDone, Item: &itemObj{
		Type: "function_call", CallID: callID, Name: name, Arguments: args,
	}})
}

func responseDoneFrame(id, status string) string {
	return frameOf(serverEvent{Type: evResponseDone, Response: &responseObj{ID: id, Status: status}})
}

// ---- shared assertions ----

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

// ---- tests ----

// TestConnect_Handshake locks the connect contract: URL model query, bearer
// + beta headers, the session.created→update→updated handshake and the
// session.update payload (formats, voice, server VAD, OpenAI tool shape).
func TestConnect_Handshake(t *testing.T) {
	srv := newMockRTServer(t)
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()),
		WithTools(ToolDef{Name: "weather", Description: "query weather", Parameters: json.RawMessage(`{"type":"object"}`)}))

	sess, answer, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if !answer.Format.Equal(pcm24k) {
		t.Fatalf("negotiated: %v", answer.Format)
	}
	ev := <-sess.Events()
	ss, ok := ev.(realtime.SessionStarted)
	if !ok {
		t.Fatalf("first event: %T", ev)
	}
	if ss.SessionID != "sess_mock_1" {
		t.Fatalf("session id must come from session.created: %q", ss.SessionID)
	}

	srv.mu.Lock()
	auth, beta, qm := srv.authHeader, srv.betaHeader, srv.queryModel
	cfgs := append([]sessionConfig(nil), srv.sessionCfgs...)
	srv.mu.Unlock()
	if auth != "Bearer sk-test" {
		t.Fatalf("authorization: %q", auth)
	}
	if beta != "realtime=v1" {
		t.Fatalf("beta header: %q", beta)
	}
	if qm != "gpt-realtime" {
		t.Fatalf("url model query: %q", qm)
	}
	if len(cfgs) != 1 {
		t.Fatalf("session updates: %d", len(cfgs))
	}
	c := cfgs[0]
	if c.InputAudioFmt != "pcm16" || c.OutputAudioFmt != "pcm16" {
		t.Fatalf("formats: %+v", c)
	}
	if c.Voice != "alloy" || c.TurnDetection["type"] != "server_vad" {
		t.Fatalf("voice/vad: %+v", c)
	}
	if len(c.Tools) != 1 || c.Tools[0]["type"] != "function" || c.Tools[0]["name"] != "weather" {
		t.Fatalf("tools: %+v", c.Tools)
	}
}

// TestFullChain_AudioToolsMetrics: one duplex audio turn with a tool call
// pushed by the server right after the handshake, driven end-to-end through
// RealtimeAgent with TurnMetrics (the same acceptance shape as the DashScope
// backend, 19.4; the agent owns the session so agent-level tests script the
// server side).
func TestFullChain_AudioToolsMetrics(t *testing.T) {
	srv := newMockRTServer(t)
	srv.autoFrames = func(n int) []string {
		return []string{
			transcriptDone("北京天气"),
			responseCreated("resp_1"),
			textDelta("北京晴"),
			audioDelta([]byte{1, 2, 3}),
			audioDelta([]byte{4, 5, 6}),
			functionCallDone("call_1", "weather", `{"city":"北京"}`),
			responseDoneFrame("resp_1", "completed"),
		}
	}
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()),
		WithTools(ToolDef{Name: "weather", Description: "query weather"}))

	metricsCh := make(chan realtime.TurnMetrics, 2)
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer:         realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}},
		OnTurnMetrics: func(m realtime.TurnMetrics) { metricsCh <- m },
	}).WithTools(realtime.ToolHandlerFunc(func(ctx context.Context, call realtime.ToolCall) ([]byte, error) {
		if call.ID != "call_1" || call.Name != "weather" {
			t.Errorf("tool call: %+v", call)
		}
		if string(call.Args) != `{"city":"北京"}` {
			t.Errorf("tool args: %s", call.Args)
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
	assertSeq(t, names, []string{
		"started", "user_done", "response_started", "text", "audio", "audio", "tool_call", "tool_result", "response_done",
	})

	select {
	case m := <-metricsCh:
		if m.ResponseID != "resp_1" || !m.Final || m.AudioChunks != 2 || m.AudioBytes != 6 || m.ToolCalls != 1 {
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

// TestBargeIn_ClientTruncation: the OpenAI card declares client truncation —
// a complete user utterance mid-response fires response.cancel on the wire
// AND cuts the local playout at the confirmed position.
func TestBargeIn_ClientTruncation(t *testing.T) {
	srv := newMockRTServer(t)
	srv.autoFrames = func(n int) []string {
		return []string{
			responseCreated("resp_9"),
			audioDelta([]byte{7, 7, 7}),
			transcriptDone("停"),
		}
	}
	srv.respond = func(evType string, e clientEvent) []string {
		if evType == evResponseCancel {
			return []string{responseDoneFrame("resp_9", "cancelled")}
		}
		return nil
	}
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()))
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}},
	})
	out, err := agent.Connect(context.Background())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	names := collect(t, out, func(ev realtime.Event) bool {
		rd, ok := ev.(realtime.ResponseDone)
		return ok && !rd.Final
	})
	assertSeq(t, names, []string{"started", "response_started", "audio", "user_done", "response_done"})

	if _, _, _, cancels := srv.snapshot(); cancels != 1 {
		t.Fatalf("response.cancel must be sent exactly once: %d", cancels)
	}
	// Client-truncation card: the local playout was cut at the confirmed
	// position (nothing confirmed yet → pending drops to zero).
	if pending := agent.Playout().Pending(); pending != 0 {
		t.Fatalf("client card must truncate local playout, pending=%d", pending)
	}
	_ = agent.Close()
}

// TestSendControls locks the control-frame shapes: audio append (base64),
// text turn (item.create + response.create), interrupt (response.cancel).
func TestSendControls(t *testing.T) {
	srv := newMockRTServer(t)
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if err := sess.SendAudio(context.Background(), []byte{0xaa}); err != nil {
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
		types, texts, audios, cancels := srv.snapshot()
		// session.update + audio append + item create + response create +
		// cancel (the empty audio chunk never hits the wire).
		if len(types) >= 5 {
			if types[1] != evAudioAppend || types[2] != evItemCreate || types[3] != evResponseCreate || types[4] != evResponseCancel {
				t.Fatalf("client frame order: %v", types)
			}
			if len(audios) != 1 || audios[0][0] != 0xaa {
				t.Fatalf("audios: %v", audios)
			}
			if len(texts) != 1 || texts[0] != "你好" {
				t.Fatalf("texts: %v", texts)
			}
			if cancels != 1 {
				t.Fatalf("cancels: %d", cancels)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not receive all client frames in time")
}

// TestTextOnly_Turn: a text turn runs item.create + response.create and
// streams a text-only reply (no audio deltas).
func TestTextOnly_Turn(t *testing.T) {
	srv := newMockRTServer(t)
	srv.respond = func(evType string, e clientEvent) []string {
		if evType != evResponseCreate {
			return nil
		}
		return []string{
			responseCreated("resp_t"),
			textDelta("你好"),
			textDelta("呀"),
			responseDoneFrame("resp_t", "completed"),
		}
	}
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if err := sess.SendText(context.Background(), "打个招呼"); err != nil {
		t.Fatalf("send text: %v", err)
	}
	names := collect(t, sess.Events(), func(ev realtime.Event) bool {
		_, ok := ev.(realtime.ResponseDone)
		return ok
	})
	assertSeq(t, names, []string{"started", "response_started", "text", "text", "response_done"})
}

// TestReconnect_Resume: a dropped connection does not dead-end the
// conversation — Reconnect re-dials and a NEW response streams through with
// metrics for both turns (no double audio: per-response de-dup holds).
func TestReconnect_Resume(t *testing.T) {
	srv := newMockRTServer(t)
	srv.closeAfterAuto = true
	srv.autoFrames = func(n int) []string {
		// Connection 1 completes resp_a then drops; connection 2 pushes
		// resp_b after the re-dial.
		if n == 1 {
			return []string{
				responseCreated("resp_a"),
				audioDelta([]byte{1, 1}),
				responseDoneFrame("resp_a", "completed"),
			}
		}
		return []string{
			responseCreated("resp_b"),
			audioDelta([]byte{2, 2}),
			responseDoneFrame("resp_b", "completed"),
		}
	}
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()))
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}},
	})

	out1, err := agent.Connect(context.Background())
	if err != nil {
		t.Fatalf("connect 1: %v", err)
	}
	for ev := range out1 {
		if rd, ok := ev.(realtime.ResponseDone); ok && rd.ResponseID == "resp_a" {
			break
		}
	}
	// The server dropped after resp_a: the stream ends.
	for range out1 {
	}

	out2, err := agent.Reconnect(context.Background())
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	names := collect(t, out2, func(ev realtime.Event) bool {
		rd, ok := ev.(realtime.ResponseDone)
		return ok && rd.ResponseID == "resp_b"
	})
	assertSeq(t, names, []string{"started", "response_started", "audio", "response_done"})

	if got := int(srv.conns.Load()); got != 2 {
		t.Fatalf("two connections expected: %d", got)
	}
	ms := agent.Metrics()
	if len(ms) != 2 || ms[0].ResponseID != "resp_a" || ms[1].ResponseID != "resp_b" {
		t.Fatalf("metrics across reconnect: %+v", ms)
	}
	if ms[0].AudioChunks != 1 || ms[1].AudioChunks != 1 {
		t.Fatalf("audio chunks across reconnect: %+v", ms)
	}
	// Both turns' audio stayed queued (nothing consumed, nothing lost to a
	// double-enqueue): 2 bytes + 2 bytes pending.
	if pending := agent.Playout().Pending(); pending != 4 {
		t.Fatalf("playout after reconnect: pending=%d", pending)
	}
	_ = agent.Close()
}

// TestError_Terminal: a mid-session error event maps to ErrorEvent and ends
// the stream.
func TestError_Terminal(t *testing.T) {
	srv := newMockRTServer(t)
	srv.autoFrames = func(n int) []string {
		return []string{frameOf(serverEvent{Type: evError, Error: &errorObj{Code: "server_error", Message: "boom"}})}
	}
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	// Skip the locally synthesized SessionStarted; the scripted error
	// follows on the wire.
	var sawError bool
	for ev := range sess.Events() {
		if _, isStart := ev.(realtime.SessionStarted); isStart {
			continue
		}
		ee, isErr := ev.(realtime.ErrorEvent)
		if !isErr {
			t.Fatalf("expected ErrorEvent, got %T", ev)
		}
		if !strings.Contains(ee.Err, "boom") || !strings.Contains(ee.Err, "server_error") {
			t.Fatalf("error message: %q", ee.Err)
		}
		sawError = true
		break
	}
	if !sawError {
		t.Fatalf("stream closed without the error event")
	}
	if _, ok := <-sess.Events(); ok {
		t.Fatalf("stream must close after ErrorEvent")
	}
}

// TestHandshake_Error: an error before session.created fails Connect with
// the server message.
func TestHandshake_Error(t *testing.T) {
	srv := newMockRTServer(t)
	srv.failStart = true
	model := NewGPTRealtime("sk-bad", WithBaseURL(srv.URL()))
	_, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}})
	if err == nil {
		t.Fatalf("connect must fail on handshake error")
	}
	if !strings.Contains(err.Error(), "Incorrect API key") {
		t.Fatalf("error must surface the server message: %v", err)
	}
}

// TestFormatNegotiation_Reject: pcm16k (wrong rate) is not silently
// transcoded — negotiation fails before dialling.
func TestFormatNegotiation_Reject(t *testing.T) {
	srv := newMockRTServer(t)
	model := NewGPTRealtime("sk-test", WithBaseURL(srv.URL()))
	pcm16 := realtime.AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}
	_, _, err := model.Connect(context.Background(), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16}})
	if err == nil {
		t.Fatalf("16kHz pcm must not be accepted for gpt-realtime (24kHz only)")
	}
}

// TestCards_Embedded: every realtime card loads with client truncation and
// duplex pcm24k — including the xAI grok card (19.6).
func TestCards_Embedded(t *testing.T) {
	cards, err := ListModelCards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("want 3 cards (gpt-realtime, 4o preview, grok), got %d", len(cards))
	}
	byID := map[string]realtime.ModelCard{}
	for _, c := range cards {
		byID[c.ID] = c
	}
	for _, c := range cards {
		if c.Truncation.Normalized() != realtime.TruncationClient || !c.Tools {
			t.Fatalf("card %s capabilities: %+v", c.ID, c)
		}
		if len(c.AudioIn) != 1 || !c.AudioIn[0].Equal(pcm24k) || len(c.AudioOut) != 1 {
			t.Fatalf("card %s formats: %+v", c.ID, c)
		}
	}
	grok, ok := byID["grok-voice-latest"]
	if !ok || grok.Provider != "xai" || grok.Model != "grok-voice-latest" {
		t.Fatalf("grok card: %+v", grok)
	}
}

// TestGrokRealtime_AliasEventNames: xAI's endpoint speaks OpenAI-compatible
// naming with the GA-era aliases (response.output_audio.delta /
// response.output_text.delta). The SAME decoder and the SAME contract test
// shape as the OpenAI path must drive a full turn — 19.6's "复用同一契约
// 测试集, 不接受仅能创建客户端".
func TestGrokRealtime_AliasEventNames(t *testing.T) {
	srv := newMockRTServer(t)
	srv.autoFrames = func(n int) []string {
		return []string{
			transcriptDone("北京天气"),
			responseCreated("resp_x1"),
			frameOf(serverEvent{Type: evTextDeltaAlt, Delta: "北京晴"}),
			frameOf(serverEvent{Type: evAudioDeltaAlt, Delta: base64.StdEncoding.EncodeToString([]byte{9, 9})}),
			functionCallDone("call_x1", "weather", `{"city":"北京"}`),
			responseDoneFrame("resp_x1", "completed"),
		}
	}
	// Default endpoint targets xAI (constructor-pinned); the mock overrides.
	model := NewGrokRealtime("xai-key", WithBaseURL(srv.URL()))
	if m := model.ModelName(); m != "grok-voice-latest" {
		t.Fatalf("model name: %s", m)
	}

	metricsCh := make(chan realtime.TurnMetrics, 1)
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer:         realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm24k}},
		OnTurnMetrics: func(m realtime.TurnMetrics) { metricsCh <- m },
	}).WithTools(realtime.ToolHandlerFunc(func(ctx context.Context, call realtime.ToolCall) ([]byte, error) {
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
	assertSeq(t, names, []string{
		"started", "user_done", "response_started", "text", "audio", "tool_call", "tool_result", "response_done",
	})
	select {
	case m := <-metricsCh:
		if m.AudioChunks != 1 || m.AudioBytes != 2 || m.ToolCalls != 1 || !m.Final {
			t.Fatalf("turn metrics: %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("OnTurnMetrics did not fire")
	}
	_ = agent.Close()
}
