// realtime/metrics_test.go — TurnMetrics collection through RealtimeAgent
// (19.4): one record per completed turn, audio/text/tool counters,
// derived latencies, and the rule that de-dropped backlog replays are NOT
// counted (metrics mirror what was actually played, not what arrived twice).
package realtime

import (
	"context"
	"testing"
)

func TestTurnMetrics_Collection(t *testing.T) {
	pcm := AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}
	script := []MockStep{
		{OnAudio: []Event{
			ResponseStarted{ResponseID: "r1"},
			TranscriptDelta{Text: "好"},
			AudioOutDelta{Sequence: 1, Data: []byte{1, 2, 3, 4}},
			AudioOutDelta{Sequence: 1, Data: []byte{1, 2, 3, 4}}, // replay: de-dup drops
			AudioOutDelta{Sequence: 2, Data: []byte{5, 6, 7, 8}},
			ToolCall{ID: "t1", Name: "sum", Args: []byte(`{"a":1}`)},
			ResponseDone{ResponseID: "r1", Final: true},
		}},
		{OnText: []Event{
			ResponseStarted{ResponseID: "r2"},
			AudioOutDelta{Sequence: 1, Data: []byte{9, 9}},
			ResponseDone{ResponseID: "r2", Final: false},
		}},
	}
	model := NewMockModel(ModelCard{}, []AudioFormat{pcm}, script)

	metricsCh := make(chan TurnMetrics, 4)
	agent := NewRealtimeAgent(model, AgentConfig{
		Offer:         NegotiateOffer{Formats: []AudioFormat{pcm}},
		OnTurnMetrics: func(m TurnMetrics) { metricsCh <- m },
	}).WithTools(ToolHandlerFunc(func(ctx context.Context, c ToolCall) ([]byte, error) {
		return []byte(`{"sum":3}`), nil
	}))

	out, err := agent.Connect(context.Background())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	sess := model.Sessions()[0]

	readUntil := func(pred func(Event) bool) {
		for ev := range out {
			if pred(ev) {
				return
			}
		}
		t.Fatalf("stream closed before expected event")
	}
	sawToolResult := false
	if err := sess.SendAudio(context.Background(), []byte{0x01}); err != nil {
		t.Fatalf("send audio: %v", err)
	}
	readUntil(func(ev Event) bool {
		if tr, ok := ev.(ToolResult); ok && tr.ID == "t1" {
			sawToolResult = true
		}
		_, done := ev.(ResponseDone)
		return done
	})
	if !sawToolResult {
		t.Fatalf("tool result must be forwarded before ResponseDone")
	}
	if err := sess.SendText(context.Background(), "next"); err != nil {
		t.Fatalf("send text: %v", err)
	}
	readUntil(func(ev Event) bool {
		rd, ok := ev.(ResponseDone)
		return ok && rd.ResponseID == "r2"
	})
	for range out { // drain to close (script exhausted)
	}

	if len(agent.Metrics()) != 2 {
		t.Fatalf("want 2 turn metrics, got %d", len(agent.Metrics()))
	}
	m1 := agent.Metrics()[0]
	if m1.ResponseID != "r1" || !m1.Final {
		t.Fatalf("turn 1 metrics: %+v", m1)
	}
	if m1.AudioChunks != 2 || m1.AudioBytes != 8 {
		t.Fatalf("replayed sequence must not be counted: chunks=%d bytes=%d", m1.AudioChunks, m1.AudioBytes)
	}
	if m1.ToolCalls != 1 {
		t.Fatalf("tool calls: %d", m1.ToolCalls)
	}
	if m1.StartedAt.IsZero() || m1.FirstTextAt.IsZero() || m1.FirstAudioAt.IsZero() || m1.DoneAt.IsZero() {
		t.Fatalf("timestamps must be stamped: %+v", m1)
	}
	if m1.TimeToFirstAudio() < 0 || m1.TimeToFirstText() < 0 {
		t.Fatalf("negative latencies: %+v", m1)
	}
	if m1.TurnDuration() < m1.TimeToFirstAudio() {
		t.Fatalf("turn duration must cover TTFA: dur=%v ttfa=%v", m1.TurnDuration(), m1.TimeToFirstAudio())
	}

	m2 := agent.Metrics()[1]
	if m2.Final || m2.AudioChunks != 1 {
		t.Fatalf("turn 2 metrics: %+v", m2)
	}
	if !m2.FirstTextAt.IsZero() || m2.TimeToFirstText() != 0 {
		t.Fatalf("textless turn must report zero text latency: %+v", m2)
	}
	if len(metricsCh) != 2 {
		t.Fatalf("OnTurnMetrics fired %d times, want 2", len(metricsCh))
	}
}

func TestTurnMetrics_ZeroValueDurations(t *testing.T) {
	m := TurnMetrics{}
	if m.TimeToFirstAudio() != 0 || m.TimeToFirstText() != 0 || m.TurnDuration() != 0 {
		t.Fatalf("zero metrics must yield zero durations: %+v", m)
	}
}
