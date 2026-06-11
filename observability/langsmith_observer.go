package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/event"
)

// LangSmithObserver subscribes to an event.Bus and forwards trace data to
// LangSmith as Run objects. It is safe for concurrent use.
type LangSmithObserver struct {
	client    *LangSmithClient
	project   string
	sessionID string

	mu       sync.Mutex
	runs     map[string]*Run // keyed by reply_id
	rootRuns map[string]*Run // keyed by reply_id
}

// NewLangSmithObserver creates an observer attached to the given client.
func NewLangSmithObserver(client *LangSmithClient, project, sessionID string) *LangSmithObserver {
	return &LangSmithObserver{
		client:    client,
		project:   project,
		sessionID: sessionID,
		runs:      make(map[string]*Run),
		rootRuns:  make(map[string]*Run),
	}
}

// Observe blocks and consumes events from the bus until ctx is done.
func (o *LangSmithObserver) Observe(ctx context.Context, bus *event.Bus) {
	id, ch, _ := bus.Subscribe()
	defer bus.Unsubscribe(id)

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			o.handle(ctx, ev)
		}
	}
}

func (o *LangSmithObserver) handle(ctx context.Context, ev event.AgentEvent) {
	switch e := ev.(type) {
	case *event.ReplyStartEvent:
		now := time.Now()
		run := &Run{
			ID:          e.ReplyID(),
			Name:        "agent-reply",
			RunType:     "chain",
			StartTime:   now,
			Inputs:      map[string]any{"agent": e.AgentName},
			Tags:        []string{"agentscope", "go"},
			SessionID:   o.sessionID,
			SessionName: o.project,
		}
		o.mu.Lock()
		o.rootRuns[e.ReplyID()] = run
		o.mu.Unlock()
		_ = o.client.CreateRun(ctx, *run)

	case *event.ReplyEndEvent:
		o.mu.Lock()
		run, ok := o.rootRuns[e.ReplyID()]
		if ok {
			now := time.Now()
			run.EndTime = &now
			delete(o.rootRuns, e.ReplyID())
			delete(o.runs, e.ReplyID())
			o.mu.Unlock()
			_ = o.client.CreateRun(ctx, *run)
		} else {
			o.mu.Unlock()
		}

	case *event.ErrorEvent:
		o.mu.Lock()
		run, ok := o.rootRuns[e.ReplyID()]
		if ok {
			now := time.Now()
			run.EndTime = &now
			run.Error = &e.Err
			delete(o.rootRuns, e.ReplyID())
			delete(o.runs, e.ReplyID())
			o.mu.Unlock()
			_ = o.client.CreateRun(ctx, *run)
		} else {
			o.mu.Unlock()
		}

	case *event.TextBlockDeltaEvent:
		// No-op: deltas are aggregated by the consumer; we trace at reply level.
	case *event.ToolCallStartEvent:
		now := time.Now()
		run := &Run{
			ID:          fmt.Sprintf("%s-tool-%d", e.ReplyID(), e.BlockIndex),
			Name:        "tool-call",
			RunType:     "tool",
			StartTime:   now,
			Inputs:      map[string]any{"tool_name": e.ToolName, "tool_call_id": e.ToolCallID},
			ParentRunID: strPtr(e.ReplyID()),
			Tags:        []string{"tool"},
			SessionID:   o.sessionID,
			SessionName: o.project,
		}
		o.mu.Lock()
		o.runs[run.ID] = run
		o.mu.Unlock()

	case *event.ToolCallEndEvent:
		key := fmt.Sprintf("%s-tool-%d", e.ReplyID(), e.BlockIndex)
		o.mu.Lock()
		run, ok := o.runs[key]
		if ok {
			now := time.Now()
			run.EndTime = &now
			delete(o.runs, key)
			o.mu.Unlock()
			_ = o.client.CreateRun(ctx, *run)
		} else {
			o.mu.Unlock()
		}
	}
}

func strPtr(s string) *string { return &s }
