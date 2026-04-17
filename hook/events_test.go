package hook

import (
	"context"
	"testing"
	"time"
)

func TestBaseEvent_Getters(t *testing.T) {
	ev := BaseEvent{Type: EventPreReasoning, Ts: time.Unix(1, 0), Agent: "a1"}
	if ev.EventType() != EventPreReasoning {
		t.Fatal("EventType mismatch")
	}
	if !ev.Timestamp().Equal(time.Unix(1, 0)) {
		t.Fatal("Timestamp mismatch")
	}
	if ev.AgentName() != "a1" {
		t.Fatal("AgentName mismatch")
	}
}

func TestStreamHookFunc_OnStreamEvent(t *testing.T) {
	called := false
	var f StreamHookFunc = func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		called = true
		return nil, nil
	}
	_, _ = f.OnStreamEvent(context.Background(), nil)
	if !called {
		t.Fatal("expected func to be called")
	}
}

func TestStreamPriorityOf(t *testing.T) {
	if StreamPriorityOf(nil) != DefaultPriority {
		t.Fatal("expected default for nil")
	}
	h := WithStreamPriority(StreamHookFunc(func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		return nil, nil
	}), 42)
	if StreamPriorityOf(h) != 42 {
		t.Fatalf("expected 42, got %d", StreamPriorityOf(h))
	}
}

func TestWithStreamPriority_Nil(t *testing.T) {
	if WithStreamPriority(nil, 10) != nil {
		t.Fatal("expected nil")
	}
}

func TestStreamPriorityHook_OnStreamEvent(t *testing.T) {
	called := false
	inner := StreamHookFunc(func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		called = true
		return &StreamHookResult{Interrupt: true}, nil
	})
	h := WithStreamPriority(inner, 5).(*streamPriorityHook)
	if h.Priority() != 5 {
		t.Fatal("priority mismatch")
	}
	res, err := h.OnStreamEvent(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called || !res.Interrupt {
		t.Fatal("unexpected result")
	}
}

func TestSortStreamHooks(t *testing.T) {
	a := WithStreamPriority(StreamHookFunc(func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		return nil, nil
	}), 200)
	b := WithStreamPriority(StreamHookFunc(func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		return nil, nil
	}), 50)
	sorted := SortStreamHooks([]StreamHook{a, b})
	if StreamPriorityOf(sorted[0]) != 50 {
		t.Fatal("order")
	}
}
