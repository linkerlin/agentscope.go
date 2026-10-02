package realtime

import (
	"context"
	"strings"
	"testing"
	"time"
)

// collect drains the agent stream until it closes or a predicate event
// arrives (returned too).
func collect(ch <-chan Event, stopAt func(Event) bool) []Event {
	var out []Event
	for ev := range ch {
		out = append(out, ev)
		if stopAt != nil && stopAt(ev) {
			return out
		}
	}
	return out
}

func isResponseDone(id string) func(Event) bool {
	return func(ev Event) bool {
		rd, ok := ev.(ResponseDone)
		return ok && rd.ResponseID == id
	}
}

// agentCard builds a realtime card with the given truncation mode.
func agentCard(trunc TruncationSupport) ModelCard {
	c := ModelCard{Truncation: trunc}
	c.ID = "agent-voice"
	c.Model = "agent-voice"
	return c
}

// TestRealtimeAgent_BargeInKeepsHeardPrefix is the acceptance's first
// clause: on a client-truncation card, a barge-in (UserTranscribed while
// the assistant speaks) cuts at the CONFIRMED playout position — the
// confirmed prefix survives, the unconfirmed tail is dropped, and the
// interrupt reaches the session.
func TestRealtimeAgent_BargeInKeepsHeardPrefix(t *testing.T) {
	script := []MockStep{
		{Auto: []Event{ResponseStarted{ResponseID: "r1"}}},
		{OnText: []Event{
			// The user's complete utterance triggers the barge-in.
			UserTranscribed{Text: "stop"},
			AudioOutDelta{Sequence: 0, Data: []byte{1}},
			AudioOutDelta{Sequence: 1, Data: []byte{2}},
			ResponseDone{ResponseID: "r1", Final: false},
		}},
	}
	m := NewMockModel(agentCard(TruncationClient), []AudioFormat{pcm16k}, script)
	a := NewRealtimeAgent(m, AgentConfig{Offer: NegotiateOffer{Formats: []AudioFormat{pcm16k}}})
	stream, err := a.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Queue the response audio, then confirm only the first chunk.
	a.Playout().Enqueue(0, []byte{1})
	a.Playout().Enqueue(1, []byte{2})
	a.Playout().Advance(1) // device confirmed chunk 0 only.

	// The user's utterance triggers the barge-in via the event stream: push
	// the turn by sending text.
	sess := m.Sessions()[0]
	_ = sess.SendText(context.Background(), "stop")

	evs := collect(stream, isResponseDone("r1"))
	// Audio deltas forwarded before the done: exactly the ones that arrived
	// after dedup.
	var deltas int
	for _, ev := range evs {
		if _, ok := ev.(AudioOutDelta); ok {
			deltas++
		}
	}
	if deltas != 2 {
		t.Fatalf("expected both deltas forwarded, got %d", deltas)
	}
	// The cut: pending (unconfirmed) bytes are dropped — 1 byte was
	// confirmed, 1 byte (chunk 1's) pending → dropped.
	if got := a.Playout().Dropped(); got != 1 {
		t.Fatalf("truncate must drop exactly the unconfirmed byte, got %d", got)
	}
	// The interrupt reached the session.
	calls := sess.Calls()
	found := false
	for _, c := range calls {
		if c == "interrupt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("interrupt must be sent to the session, calls=%v", calls)
	}
}

// TestRealtimeAgent_ServerTruncationDelegates: on a server-truncation card
// the barge-in only sends Interrupt — the local queue is NOT truncated (the
// server cuts and confirms via Interrupted).
func TestRealtimeAgent_ServerTruncationDelegates(t *testing.T) {
	script := []MockStep{
		{Auto: []Event{ResponseStarted{ResponseID: "r1"}}},
		{OnText: []Event{UserTranscribed{Text: "stop"}}},
	}
	m := NewMockModel(agentCard(TruncationServer), []AudioFormat{pcm16k}, script)
	a := NewRealtimeAgent(m, AgentConfig{Offer: NegotiateOffer{Formats: []AudioFormat{pcm16k}}})
	stream, err := a.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a.Playout().Enqueue(0, []byte{1, 2, 3})
	sess := m.Sessions()[0]
	_ = sess.SendText(context.Background(), "stop")
	collect(stream, func(ev Event) bool {
		_, ok := ev.(UserTranscribed)
		return ok
	})
	if a.Playout().Dropped() != 0 {
		t.Fatal("server-truncation card must not cut locally")
	}
}

// TestRealtimeAgent_ReconnectNoLossNoDuplication is the backlog clause: a
// dropped stream re-dials; audio already delivered is not replayed into the
// playout twice, and the resumed stream's audio continues from where the
// old one left off.
func TestRealtimeAgent_ReconnectNoLossNoDuplication(t *testing.T) {
	// Session 1 script: response r1 with seq 0 delivered, then the stream
	// drops (script ends → mock closes cleanly, standing in for a dropped
	// connection mid-response).
	script1 := []MockStep{
		{Auto: []Event{
			ResponseStarted{ResponseID: "r1"},
			AudioOutDelta{Sequence: 0, Data: []byte{1}},
			// No ResponseDone: the turn was cut by the drop.
		}},
	}
	m := NewMockModel(agentCard(TruncationClient), []AudioFormat{pcm16k}, script1)
	a := NewRealtimeAgent(m, AgentConfig{Offer: NegotiateOffer{Formats: []AudioFormat{pcm16k}}})
	stream1, err := a.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Drain until the drop (stream closes).
	for range stream1 {
	}
	if a.Playout().Pending() != 1 {
		t.Fatalf("seq0 must be queued, pending=%d", a.Playout().Pending())
	}

	// Reconnect: the backend (replaying its backlog) resends seq 0 (dup)
	// then continues with seq 1 (the part the agent never saw).
	script2 := []MockStep{
		{Auto: []Event{
			ResponseStarted{ResponseID: "r1"},
			AudioOutDelta{Sequence: 0, Data: []byte{1}}, // replay: must be dropped
			AudioOutDelta{Sequence: 1, Data: []byte{2}}, // continuation: must land
			ResponseDone{ResponseID: "r1", Final: true},
		}},
	}
	m2script(script2, m)
	stream2, err := a.Reconnect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(stream2, isResponseDone("r1"))
	forwarded := 0
	for _, ev := range evs {
		if _, ok := ev.(AudioOutDelta); ok {
			forwarded++
		}
	}
	// Only the continuation delta is new; the replayed one is swallowed.
	if forwarded != 1 {
		t.Fatalf("replay must be de-duplicated: forwarded=%d want 1", forwarded)
	}
	// Playout holds seq0 (old) + seq1 (new) — nothing lost, nothing twice.
	if a.Playout().Pending() != 2 {
		t.Fatalf("backlog wrong: pending=%d want 2", a.Playout().Pending())
	}
}

// m2script swaps the mock's script for the reconnect scenario (the mock
// keeps one script per model; tests reuse the model across re-dials).
func m2script(script []MockStep, m *MockModel) {
	m.mu.Lock()
	m.script = script
	m.mu.Unlock()
}

// TestRealtimeAgent_ToolConfirmTimeoutResumes is the tool clause: a tool
// handler that stalls forever must not dead-end the session — after
// ToolConfirmTimeout the conversation resumes with an error ToolResult.
func TestRealtimeAgent_ToolConfirmTimeoutResumes(t *testing.T) {
	script := []MockStep{
		{Auto: []Event{
			ResponseStarted{ResponseID: "r1"},
			ToolCall{ID: "tc1", Name: "deploy", Args: []byte(`{}`)},
		}},
	}
	m := NewMockModel(agentCard(TruncationClient), []AudioFormat{pcm16k}, script)
	a := NewRealtimeAgent(m, AgentConfig{
		Offer:              NegotiateOffer{Formats: []AudioFormat{pcm16k}},
		ToolConfirmTimeout: 80 * time.Millisecond,
	})
	stalled := ToolHandlerFunc(func(ctx context.Context, call ToolCall) ([]byte, error) {
		<-ctx.Done() // a confirmation that never arrives
		return nil, ctx.Err()
	})
	a.WithTools(stalled)

	start := time.Now()
	stream, err := a.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var result *ToolResult
	for ev := range stream {
		if tr, ok := ev.(ToolResult); ok {
			result = &tr
			break
		}
	}
	if result == nil {
		t.Fatal("timeout must synthesize a ToolResult")
	}
	if result.ID != "tc1" || !strings.Contains(result.Err, "timeout") {
		t.Fatalf("result wrong: %+v", result)
	}
	if elapsed := time.Since(start); elapsed < 70*time.Millisecond {
		t.Fatalf("resume must respect the timeout, resumed at %v", elapsed)
	}
	// The session did NOT dead-end: the pump still forwards the terminal
	// stream when it comes (here: script-exhaust close).
	for range stream {
	}
}

// TestRealtimeAgent_ToolResultFlows: a healthy handler's outcome flows as
// ToolResult after the ToolCall event.
func TestRealtimeAgent_ToolResultFlows(t *testing.T) {
	script := []MockStep{
		{Auto: []Event{
			ResponseStarted{ResponseID: "r1"},
			ToolCall{ID: "tc9", Name: "lookup", Args: []byte(`{}`)},
			ResponseDone{ResponseID: "r1", Final: true},
		}},
	}
	m := NewMockModel(agentCard(TruncationClient), []AudioFormat{pcm16k}, script)
	a := NewRealtimeAgent(m, AgentConfig{Offer: NegotiateOffer{Formats: []AudioFormat{pcm16k}}})
	a.WithTools(ToolHandlerFunc(func(ctx context.Context, call ToolCall) ([]byte, error) {
		return []byte(`{"ok":true}`), nil
	}))
	stream, err := a.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(stream, isResponseDone("r1"))
	var sawCall, sawResult bool
	for _, ev := range evs {
		if tc, ok := ev.(ToolCall); ok && tc.ID == "tc9" {
			sawCall = true
		}
		if tr, ok := ev.(ToolResult); ok && tr.ID == "tc9" && string(tr.Output) == `{"ok":true}` {
			sawResult = true
		}
	}
	if !sawCall || !sawResult {
		t.Fatalf("call/result flow broken: call=%v result=%v", sawCall, sawResult)
	}
}
