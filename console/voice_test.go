// console/voice_test.go — the 19.7 acceptance suite: a scripted MockModel is
// the fake transport; fake mic/sink replace device I/O (real capture and
// playback live only in the manual smoke example). Covers the three
// acceptance clauses: text↔voice switching (text input available in BOTH
// modes), barge-in (manual + the VAD speech-start path) and the HITL tool
// confirmation bridge.
package console

import (
	"context"
	"encoding/binary"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/realtime"
)

var pcm16k = realtime.AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}

// ---- fakes ----

type fakeMic struct {
	mu    sync.Mutex
	live  bool
	emit  func(pcm []byte)
	stops int
}

func (f *fakeMic) Start(ctx context.Context, emit func(pcm []byte)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emit = emit
	f.live = true
	return nil
}

func (f *fakeMic) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.live = false
	f.stops++
	return nil
}

func (f *fakeMic) running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.live
}

// feed simulates one captured frame (little-endian PCM bytes).
func (f *fakeMic) feed(samples []int16) {
	b := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(s))
	}
	f.mu.Lock()
	emit := f.emit
	f.mu.Unlock()
	if emit != nil {
		emit(b)
	}
}

type fakeSink struct {
	mu     sync.Mutex
	chunks [][]byte
}

func (f *fakeSink) Play(seq int, chunk []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks = append(f.chunks, chunk)
}

func (f *fakeSink) played() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.chunks)
}

// waitUntil polls cond until it holds or the deadline passes.
func waitUntil(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// ---- acceptance: text↔voice switching ----

// TestVoiceSession_TextVoiceSwitching: text input works in both modes; the
// mic starts on voice and stops back on text; mic frames reach the session;
// voice mode without a mic source refuses (not silently degraded).
func TestVoiceSession_TextVoiceSwitching(t *testing.T) {
	turn := func(text string) []realtime.Event {
		return []realtime.Event{
			realtime.ResponseStarted{ResponseID: "r"},
			realtime.TranscriptDelta{Text: text},
			realtime.ResponseDone{ResponseID: "r", Final: true},
		}
	}
	// NOTE: the mock consumes CONSECUTIVE same-type steps on one control
	// call — the OnAudio step between the two OnText steps keeps the two
	// typed turns separately triggerable.
	model := realtime.NewMockModel(realtime.ModelCard{}, []realtime.AudioFormat{pcm16k}, []realtime.MockStep{
		{OnText: turn("好")}, // SubmitText from text mode
		{OnAudio: []realtime.Event{realtime.ResponseDone{ResponseID: "r2", Final: true}}},
		{OnText: turn("呀")}, // SubmitText from VOICE mode (both channels live)
	})

	var mu sync.Mutex
	var texts int
	mic := &fakeMic{}
	sink := &fakeSink{}
	sess, err := StartVoiceSession(context.Background(), VoiceOptions{
		Model: model,
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
		Mic:   mic,
		Sink:  sink,
		OnText: func(string) {
			mu.Lock()
			texts++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sess.Close()

	if sess.Mode() != VoiceModeText {
		t.Fatalf("default mode: %s", sess.Mode())
	}
	// Text mode: typed input drives a turn.
	if err := sess.SubmitText("hi"); err != nil {
		t.Fatalf("submit text: %v", err)
	}
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return texts == 1
	}, "text turn 1")

	// Switch to voice: mic runs, mic frames reach the session, typed input
	// STILL works.
	if err := sess.SetMode(VoiceModeVoice); err != nil {
		t.Fatalf("set voice mode: %v", err)
	}
	if !mic.running() {
		t.Fatal("voice mode must start the mic")
	}
	mic.feed(realtime.ToneSynth(320, 8000))
	waitUntil(t, func() bool {
		for _, c := range model.Sessions()[0].Calls() {
			if c == "audio" {
				return true
			}
		}
		return false
	}, "mic frame reaching the session")
	if err := sess.SubmitText("typed while in voice mode"); err != nil {
		t.Fatalf("submit text in voice mode: %v", err)
	}
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return texts == 2
	}, "text turn 2 in voice mode")

	// Back to text: mic stopped.
	if err := sess.SetMode(VoiceModeText); err != nil {
		t.Fatalf("set text mode: %v", err)
	}
	if mic.running() {
		t.Fatal("text mode must stop the mic")
	}

	// No mic configured: voice mode refuses loudly.
	sess2, err := StartVoiceSession(context.Background(), VoiceOptions{
		Model: realtime.NewMockModel(realtime.ModelCard{}, nil, nil),
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
	})
	if err != nil {
		t.Fatalf("start 2: %v", err)
	}
	defer sess2.Close()
	if err := sess2.SetMode(VoiceModeVoice); err == nil {
		t.Fatal("voice mode without a mic must refuse")
	}
}

// ---- acceptance: barge-in (manual + VAD speech-start) ----

// TestVoiceSession_BargeIn: while the assistant speaks, the VAD speech-start
// edge from the mic triggers the interrupt — the session receives the
// interrupt control call, playback is cut (client-truncation card) and the
// turn closes as NOT final. The manual path (VoiceSession.BargeIn) fires the
// same cut.
func TestVoiceSession_BargeIn(t *testing.T) {
	model := realtime.NewMockModel(realtime.ModelCard{}, []realtime.AudioFormat{pcm16k}, []realtime.MockStep{
		{OnText: []realtime.Event{
			realtime.ResponseStarted{ResponseID: "r1"},
			realtime.AudioOutDelta{Sequence: 1, Data: []byte{1, 2, 3, 4}},
		}},
		{OnInterrupt: []realtime.Event{
			realtime.ResponseDone{ResponseID: "r1", Final: false},
		}},
	})
	mic := &fakeMic{}
	sink := &fakeSink{}
	metricsCh := make(chan realtime.TurnMetrics, 2)
	vad := realtime.NewVAD(realtime.VADConfig{Threshold: 100, MinSpeechFrames: 1, HangoverFrames: 2})
	sess, err := StartVoiceSession(context.Background(), VoiceOptions{
		Model: model,
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
		Mic:   mic,
		Sink:  sink,
		VAD:   vad,
		OnTurnMetrics: func(m realtime.TurnMetrics) {
			metricsCh <- m
		},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sess.Close()

	if err := sess.SetMode(VoiceModeVoice); err != nil {
		t.Fatalf("voice mode: %v", err)
	}
	// Start a turn; the assistant speaks one audio chunk.
	if err := sess.SubmitText("tell me something"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitUntil(t, func() bool { return sink.played() == 1 }, "audio chunk played")

	// The user starts talking over the assistant: VAD speech-start →
	// barge-in (auto path, no manual call).
	mic.feed(realtime.ToneSynth(320, 20000))
	waitUntil(t, func() bool {
		for _, c := range model.Sessions()[0].Calls() {
			if c == "interrupt" {
				return true
			}
		}
		return false
	}, "interrupt reaching the session")

	select {
	case m := <-metricsCh:
		if m.Final {
			t.Fatalf("barge-in turn must not be final: %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("turn metrics missing")
	}

	// Manual path: same interrupt semantics.
	sess.BargeIn()
	waitUntil(t, func() bool {
		n := 0
		for _, c := range model.Sessions()[0].Calls() {
			if c == "interrupt" {
				n++
			}
		}
		return n >= 2
	}, "manual barge-in reaching the session")
}

// ---- acceptance: HITL tool confirmation ----

// TestVoiceSession_HITLToolConfirm: a mid-conversation tool call surfaces to
// the ConfirmTool bridge; approve delivers the output, decline delivers a
// refusal ToolResult — both resume the conversation.
func TestVoiceSession_HITLToolConfirm(t *testing.T) {
	model := realtime.NewMockModel(realtime.ModelCard{}, []realtime.AudioFormat{pcm16k}, []realtime.MockStep{
		{OnText: []realtime.Event{
			realtime.ResponseStarted{ResponseID: "r1"},
			realtime.ToolCall{ID: "t1", Name: "weather", Args: []byte(`{"city":"北京"}`)},
			realtime.ResponseDone{ResponseID: "r1", Final: true},
		}},
		// An OnInterrupt step separates the two OnText steps (the mock
		// consumes consecutive same-type steps on ONE control call): the
		// manual BargeIn between the two typed turns advances it.
		{OnInterrupt: []realtime.Event{realtime.ResponseDone{ResponseID: "rx", Final: true}}},
		{OnText: []realtime.Event{
			realtime.ResponseStarted{ResponseID: "r2"},
			realtime.ToolCall{ID: "t2", Name: "delete_file", Args: []byte(`{"path":"/tmp/x"}`)},
			realtime.ResponseDone{ResponseID: "r2", Final: true},
		}},
	})

	var mu sync.Mutex
	var results []realtime.ToolResult
	var seen []realtime.ToolCall
	sess, err := StartVoiceSession(context.Background(), VoiceOptions{
		Model: model,
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
		ConfirmTool: func(call realtime.ToolCall) (bool, []byte) {
			mu.Lock()
			seen = append(seen, call)
			mu.Unlock()
			if call.ID == "t1" {
				return true, []byte(`{"cond":"sunny"}`)
			}
			return false, nil // declined
		},
		OnToolResult: func(r realtime.ToolResult) {
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sess.Close()

	if err := sess.SubmitText("1"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(results) >= 1
	}, "approved tool result")
	// Advance the separator step (also exercises the manual barge-in path).
	sess.BargeIn()
	if err := sess.SubmitText("2"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(results) >= 2
	}, "declined tool result")

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0].Name != "weather" || seen[1].Name != "delete_file" {
		t.Fatalf("confirmed calls: %+v", seen)
	}
	if string(results[0].Output) != `{"cond":"sunny"}` || results[0].Err != "" {
		t.Fatalf("approved result: %+v", results[0])
	}
	if results[1].Err == "" || !strings.Contains(results[1].Err, "declined") {
		t.Fatalf("declined result must carry the refusal: %+v", results[1])
	}
}
