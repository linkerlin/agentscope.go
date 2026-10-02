// realtime/dashscope/dashscope_test.go — local WebSocket mock contract tests
// (19.4 acceptance): qwen-omni / qwen-audio / CosyVoice tasks run audio,
// tools and metrics end-to-end against a scripted fake DashScope endpoint.
// The mock locks the repo-declared frame contract (header.action/event +
// payload output types); the live-service smoke test is opt-in
// (smoke_test.go).
package dashscope

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
)

var pcm16k = realtime.AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}

func ctx(t *testing.T) context.Context { return context.Background() }

// ---- mock DashScope task endpoint ----

type mockTaskServer struct {
	srv *httptest.Server

	mu             sync.Mutex
	authHeader     string
	inspHeader     string
	runTaskPayload clientPayload
	actions        []string
	texts          []string
	audios         [][]byte
	controls       []string
	failStart      bool

	// respond returns raw server frames to write for one non-run-task
	// client frame (nil = nothing).
	respond func(action string, f clientFrame) []string
	// auto frames are written right after task-started.
	auto []string
}

func newMockTaskServer(t *testing.T) *mockTaskServer {
	t.Helper()
	m := &mockTaskServer{}
	up := websocket.Upgrader{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		m.mu.Lock()
		m.authHeader = r.Header.Get("Authorization")
		m.inspHeader = r.Header.Get("X-DashScope-DataInspection")
		m.mu.Unlock()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var f clientFrame
			if err := json.Unmarshal(data, &f); err != nil {
				continue
			}
			m.record(f)
			var frames []string
			switch f.Header.Action {
			case ActionRunTask:
				if m.failStart {
					frames = []string{taskFailedFrame(f.Header.TaskID, "InvalidParameter", "bad parameters")}
				} else {
					frames = append([]string{taskStartedFrame(f.Header.TaskID)}, m.auto...)
				}
			default:
				if m.respond != nil {
					frames = m.respond(f.Header.Action, f)
				}
				// Real servers answer finish-task with task-finished;
				// default it so Close paths don't depend on every test
				// scripting it (a test's own respond wins).
				if f.Header.Action == ActionFinishTask && len(frames) == 0 {
					frames = []string{taskFinishedFrame(f.Header.TaskID)}
				}
			}
			for _, fr := range frames {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(fr)); err != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(func() { m.srv.Close() })
	return m
}

func (m *mockTaskServer) URL() string { return m.srv.URL }

func (m *mockTaskServer) record(f clientFrame) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actions = append(m.actions, f.Header.Action)
	switch f.Header.Action {
	case ActionRunTask:
		m.runTaskPayload = f.Payload
	case ActionContinueTask:
		if f.Payload.Input != nil {
			if f.Payload.Input.Text != "" {
				m.texts = append(m.texts, f.Payload.Input.Text)
			}
			if f.Payload.Input.Audio != "" {
				raw, err := base64.StdEncoding.DecodeString(f.Payload.Input.Audio)
				if err == nil {
					m.audios = append(m.audios, raw)
				}
			}
		}
	case ActionTaskControl:
		m.controls = append(m.controls, f.Payload.Command)
	}
}

func (m *mockTaskServer) snapshot() (actions, texts []string, audios [][]byte, controls []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.actions...), append([]string(nil), m.texts...),
		append([][]byte(nil), m.audios...), append([]string(nil), m.controls...)
}

// ---- frame builders ----

func taskStartedFrame(taskID string) string {
	var f serverFrame
	f.Header.Event = EventTaskStarted
	f.Header.TaskID = taskID
	b, _ := json.Marshal(f)
	return string(b)
}

func taskFailedFrame(taskID, code, msg string) string {
	var f serverFrame
	f.Header.Event = EventTaskFailed
	f.Header.TaskID = taskID
	f.Header.ErrorCode = code
	f.Header.ErrorMessage = msg
	b, _ := json.Marshal(f)
	return string(b)
}

func taskFinishedFrame(taskID string) string {
	var f serverFrame
	f.Header.Event = EventTaskFinished
	f.Header.TaskID = taskID
	b, _ := json.Marshal(f)
	return string(b)
}

func resultFrame(taskID string, o serverOutput) string {
	var f serverFrame
	f.Header.Event = EventResultGenerated
	f.Header.TaskID = taskID
	f.Payload.Output = o
	b, _ := json.Marshal(f)
	return string(b)
}

func mustInt(t *testing.T, v any) int {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("parameter not a number: %T %v", v, v)
	}
	return int(f)
}

// typeName renders an event for sequence assertions.
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

// collect reads the stream until pred matches (exclusive) or a terminal
// event lands, returning the event names seen (terminal included).
func collect(t *testing.T, out <-chan realtime.Event, pred func(realtime.Event) bool) []string {
	t.Helper()
	var names []string
	for ev := range out {
		if realtime.Terminal(ev) {
			return append(names, typeName(ev))
		}
		if pred(ev) {
			return append(names, typeName(ev))
		}
		names = append(names, typeName(ev))
	}
	return names
}

// ---- tests ----

// TestConnect_Handshake locks the run-task contract: endpoint auth headers,
// task/function/model wiring and the negotiated input format parameters.
func TestConnect_HandshakeAndRunTask(t *testing.T) {
	srv := newMockTaskServer(t)
	model := NewQwenOmniRealtime("test-key", WithBaseURL(srv.URL()))

	sess, answer, err := model.Connect(ctx(t), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if !answer.Format.Equal(pcm16k) {
		t.Fatalf("negotiated format: %v", answer.Format)
	}
	ev := <-sess.Events()
	ss, ok := ev.(realtime.SessionStarted)
	if !ok {
		t.Fatalf("first event must be SessionStarted, got %T", ev)
	}
	if ss.SessionID == "" || !ss.Format.Equal(pcm16k) {
		t.Fatalf("session started: %+v", ss)
	}

	srv.mu.Lock()
	pl := srv.runTaskPayload
	auth, insp := srv.authHeader, srv.inspHeader
	srv.mu.Unlock()
	if auth != "bearer test-key" {
		t.Fatalf("authorization header: %q", auth)
	}
	if insp != "enable" {
		t.Fatalf("data inspection header: %q", insp)
	}
	if pl.Task != "qwen-omni-realtime" || pl.Function != "asr+llm+tts" || pl.Model != "qwen-omni-turbo-realtime" {
		t.Fatalf("run-task payload: %+v", pl)
	}
	if pl.Parameters["format"] != "pcm" || mustInt(t, pl.Parameters["sample_rate"]) != 16000 || mustInt(t, pl.Parameters["channels"]) != 1 {
		t.Fatalf("run-task audio parameters: %+v", pl.Parameters)
	}
	if actions, _, _, _ := srv.snapshot(); actions[0] != ActionRunTask {
		t.Fatalf("first action must be run-task: %v", actions)
	}
}

// TestQwenOmni_FullChain_AudioToolsMetrics is the 19.4 acceptance core:
// qwen-omni over the mock WebSocket runs one full audio turn (ASR partial +
// utterance, transcript, two audio chunks, one tool call) driven end-to-end
// through RealtimeAgent, with TurnMetrics collected. The turn is pushed by
// the server right after task-started (auto frames) — the agent owns no
// input surface, so agent-level tests script the server side.
func TestQwenOmni_FullChain_AudioToolsMetrics(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.auto = []string{
		resultFrame("", serverOutput{Type: OutInputText, Text: "北"}),
		resultFrame("", serverOutput{Type: OutInputText, Text: "北京天气", Definite: true}),
		resultFrame("", serverOutput{Type: OutResponseStarted, ResponseID: "r1"}),
		resultFrame("", serverOutput{Type: OutText, Text: "北京晴"}),
		resultFrame("", serverOutput{Type: OutAudio, Data: base64.StdEncoding.EncodeToString([]byte{1, 2, 3}), Sequence: 1}),
		resultFrame("", serverOutput{Type: OutAudio, Data: base64.StdEncoding.EncodeToString([]byte{4, 5, 6}), Sequence: 2}),
		resultFrame("", serverOutput{Type: OutToolCalls, ToolCalls: []serverToolCall{
			{ID: "t1", Name: "weather", Arguments: json.RawMessage(`"{\"city\":\"北京\"}"`)},
		}}),
		resultFrame("", serverOutput{Type: OutResponseDone, ResponseID: "r1", Final: true}),
	}
	model := NewQwenOmniRealtime("test-key", WithBaseURL(srv.URL()),
		WithTools(ToolDef{Name: "weather", Description: "query weather"}))

	metricsCh := make(chan realtime.TurnMetrics, 2)
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer:         realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
		OnTurnMetrics: func(m realtime.TurnMetrics) { metricsCh <- m },
	}).WithTools(realtime.ToolHandlerFunc(func(c context.Context, call realtime.ToolCall) ([]byte, error) {
		if call.Name != "weather" {
			t.Errorf("tool name: %s", call.Name)
		}
		if string(call.Args) != `{"city":"北京"}` {
			t.Errorf("tool args (must be unwrapped from the JSON string): %s", call.Args)
		}
		return []byte(`{"cond":"sunny"}`), nil
	}))

	out, err := agent.Connect(ctx(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	names := collect(t, out, func(ev realtime.Event) bool {
		rd, ok := ev.(realtime.ResponseDone)
		return ok && rd.Final
	})
	want := []string{"started", "input_delta", "user_done", "response_started", "text", "audio", "audio", "tool_call", "tool_result", "response_done"}
	if len(names) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("event sequence[%d]: got %s want %s (full: %v)", i, names[i], want[i], names)
		}
	}

	// One turn, 2 audio chunks, 1 tool call, final, sane latencies.
	select {
	case m := <-metricsCh:
		if m.ResponseID != "r1" || !m.Final || m.AudioChunks != 2 || m.AudioBytes != 6 || m.ToolCalls != 1 {
			t.Fatalf("turn metrics: %+v", m)
		}
		if m.TimeToFirstAudio() < 0 || m.TurnDuration() < m.TimeToFirstAudio() {
			t.Fatalf("turn metrics latencies: %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("OnTurnMetrics did not fire")
	}

	_ = agent.Close()
	if actions, _, _, _ := srv.snapshot(); actions[len(actions)-1] != ActionFinishTask {
		t.Fatalf("close must send finish-task, actions=%v", actions)
	}
}

// TestBargeIn_ServerTruncation: a complete user utterance while the
// assistant speaks triggers the interrupt path — task-control interrupt on
// the wire, the Interrupted event forwarded, and (server card) NO local
// playout truncation: the queued audio stays.
func TestBargeIn_ServerTruncation(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.auto = []string{
		resultFrame("", serverOutput{Type: OutResponseStarted, ResponseID: "r9"}),
		resultFrame("", serverOutput{Type: OutAudio, Data: base64.StdEncoding.EncodeToString([]byte{7, 7}), Sequence: 1}),
		resultFrame("", serverOutput{Type: OutInputText, Text: "停", Definite: true}),
	}
	srv.respond = func(action string, f clientFrame) []string {
		if action == ActionTaskControl {
			return []string{
				resultFrame(f.Header.TaskID, serverOutput{Type: OutInterrupted}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutResponseDone, ResponseID: "r9", Final: false}),
			}
		}
		return nil
	}
	model := NewQwenOmniRealtime("test-key", WithBaseURL(srv.URL()))
	agent := realtime.NewRealtimeAgent(model, realtime.AgentConfig{
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
	})
	out, err := agent.Connect(ctx(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	names := collect(t, out, func(ev realtime.Event) bool {
		_, ok := ev.(realtime.Interrupted)
		return ok
	})
	want := []string{"started", "response_started", "audio", "user_done", "interrupted"}
	if len(names) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("event sequence[%d]: got %s want %s (full: %v)", i, names[i], want[i], names)
		}
	}
	if _, _, _, controls := srv.snapshot(); len(controls) != 1 || controls[0] != CommandInterrupt {
		t.Fatalf("interrupt must be task-control interrupt, controls=%v", controls)
	}
	// Server-truncation card: the local playout keeps its queue (the server
	// declares the cut; nothing local is dropped). Pending counts BYTES:
	// one 2-byte chunk stays unconfirmed.
	if pending := agent.Playout().Pending(); pending != 2 {
		t.Fatalf("server card must not truncate local playout, pending=%d", pending)
	}
	_ = agent.Close()
}

// TestSendAudio_RoundTrip locks the continue-task wire shapes: base64 audio
// byte-exact, text passthrough, task-control interrupt command.
func TestSendAudio_RoundTrip(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.respond = func(action string, f clientFrame) []string { return nil }
	model := NewQwenOmniRealtime("test-key", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(ctx(t), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	chunk := []byte{0xaa, 0xbb, 0xcc}
	if err := sess.SendAudio(ctx(t), chunk); err != nil {
		t.Fatalf("send audio: %v", err)
	}
	if err := sess.SendText(ctx(t), "北京天气"); err != nil {
		t.Fatalf("send text: %v", err)
	}
	if err := sess.Interrupt(ctx(t)); err != nil {
		t.Fatalf("interrupt: %v", err)
	}
	// Empty audio chunks are dropped client-side (no protocol noise).
	if err := sess.SendAudio(ctx(t), nil); err != nil {
		t.Fatalf("empty audio: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		actions, texts, audios, controls := srv.snapshot()
		// run-task + audio + text continue-tasks + task-control (the empty
		// audio chunk never reaches the wire).
		if len(actions) >= 4 {
			if len(audios) != 1 || string(audios[0]) != "\xaa\xbb\xcc" {
				t.Fatalf("audio round-trip: %v", audios)
			}
			if len(texts) != 1 || texts[0] != "北京天气" {
				t.Fatalf("texts: %v", texts)
			}
			if len(controls) != 1 || controls[0] != CommandInterrupt {
				t.Fatalf("controls: %v", controls)
			}
			_ = sess.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not receive all control frames in time")
}

// TestCosyVoice_TextToSpeech: the TTS task is text-in / audio-out —
// SendText drives audio deltas, function wiring is "tts".
func TestCosyVoice_TextToSpeech(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.respond = func(action string, f clientFrame) []string {
		if action == ActionContinueTask && f.Payload.Input != nil && f.Payload.Input.Text != "" {
			return []string{
				resultFrame(f.Header.TaskID, serverOutput{Type: OutAudio, Data: base64.StdEncoding.EncodeToString([]byte{1, 1}), Sequence: 1}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutAudio, Data: base64.StdEncoding.EncodeToString([]byte{2, 2}), Sequence: 2}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutResponseDone, ResponseID: "s1", Final: true}),
			}
		}
		return nil
	}
	model := NewCosyVoiceRealtime("test-key", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(ctx(t), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if err := sess.SendText(ctx(t), "今天天气不错"); err != nil {
		t.Fatalf("send text: %v", err)
	}
	names := collect(t, sess.Events(), func(ev realtime.Event) bool {
		_, ok := ev.(realtime.ResponseDone)
		return ok
	})
	want := []string{"started", "audio", "audio", "response_done"}
	if len(names) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("event sequence[%d]: got %s want %s", i, names[i], want[i])
		}
	}
	srv.mu.Lock()
	fn := srv.runTaskPayload.Function
	srv.mu.Unlock()
	if fn != "tts" {
		t.Fatalf("cosyvoice function wiring: %q", fn)
	}
}

// TestQwenAudio_TextReply: the asr+llm task replies in text only — no audio
// deltas in the stream.
func TestQwenAudio_TextReply(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.respond = func(action string, f clientFrame) []string {
		if action == ActionContinueTask && f.Payload.Input != nil && f.Payload.Input.Audio != "" {
			return []string{
				resultFrame(f.Header.TaskID, serverOutput{Type: OutInputText, Text: "你好", Definite: true}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutResponseStarted, ResponseID: "a1"}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutText, Text: "你"}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutText, Text: "好呀"}),
				resultFrame(f.Header.TaskID, serverOutput{Type: OutResponseDone, ResponseID: "a1", Final: true}),
			}
		}
		return nil
	}
	model := NewQwenAudioRealtime("test-key", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(ctx(t), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	if err := sess.SendAudio(ctx(t), []byte{9, 9, 9}); err != nil {
		t.Fatalf("send audio: %v", err)
	}
	names := collect(t, sess.Events(), func(ev realtime.Event) bool {
		_, ok := ev.(realtime.ResponseDone)
		return ok
	})
	want := []string{"started", "user_done", "response_started", "text", "text", "response_done"}
	if len(names) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("event sequence[%d]: got %s want %s", i, names[i], want[i])
		}
	}
}

// TestTaskFailed_AtHandshake: a task-failed first frame surfaces the error
// message from Connect.
func TestTaskFailed_AtHandshake(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.failStart = true
	model := NewQwenOmniRealtime("test-key", WithBaseURL(srv.URL()))
	_, _, err := model.Connect(ctx(t), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err == nil {
		t.Fatalf("connect must fail on task-failed")
	}
	if got := err.Error(); !strings.Contains(got, "bad parameters") {
		t.Fatalf("error must surface the server message: %v", got)
	}
}

// TestClose_FinishTask_SessionClosed: Close sends finish-task, the server's
// task-finished maps to SessionClosed and the stream ends.
func TestClose_FinishTask_SessionClosed(t *testing.T) {
	srv := newMockTaskServer(t)
	srv.respond = func(action string, f clientFrame) []string {
		if action == ActionFinishTask {
			return []string{taskFinishedFrame(f.Header.TaskID)}
		}
		return nil
	}
	model := NewQwenOmniRealtime("test-key", WithBaseURL(srv.URL()))
	sess, _, err := model.Connect(ctx(t), realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	// SessionStarted arrives; then Close drains to SessionClosed.
	select {
	case ev := <-sess.Events():
		if _, ok := ev.(realtime.SessionStarted); !ok {
			t.Fatalf("first event: %T", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no session started event")
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("close must be idempotent: %v", err)
	}

	select {
	case ev, ok := <-sess.Events():
		if !ok {
			t.Fatalf("stream closed without SessionClosed")
		}
		if _, ok := ev.(realtime.SessionClosed); !ok {
			t.Fatalf("expected SessionClosed, got %T", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no terminal event after close")
	}
	if _, ok := <-sess.Events(); ok {
		t.Fatalf("stream must close after SessionClosed")
	}
	actions, _, _, _ := srv.snapshot()
	if actions[len(actions)-1] != ActionFinishTask {
		t.Fatalf("actions after close: %v", actions)
	}
}

// TestCards_Embedded: three DashScope realtime cards load with sane
// capability declarations.
func TestCards_Embedded(t *testing.T) {
	cards, err := ListModelCards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	byID := map[string]realtime.ModelCard{}
	for _, c := range cards {
		byID[c.ID] = c
	}
	for _, id := range []string{"qwen-omni-turbo-realtime", "qwen-audio-realtime", "cosyvoice-v2-realtime"} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("missing card %s (have %v)", id, byID)
		}
	}
	omni := byID["qwen-omni-turbo-realtime"]
	if omni.Truncation.Normalized() != realtime.TruncationServer || !omni.Tools {
		t.Fatalf("omni card capabilities: %+v", omni)
	}
	if len(omni.AudioIn) != 1 || !omni.AudioIn[0].Equal(pcm16k) {
		t.Fatalf("omni audio_in: %+v", omni.AudioIn)
	}
	cosy := byID["cosyvoice-v2-realtime"]
	if cosy.Truncation.Normalized() != realtime.TruncationNone || cosy.Tools {
		t.Fatalf("cosyvoice card capabilities: %+v", cosy)
	}
}
