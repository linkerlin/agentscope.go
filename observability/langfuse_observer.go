package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/event"
)

// LangfuseObserver subscribes to an event.Bus and forwards agent trace data to
// Langfuse as trace/span/generation events. It is the Langfuse counterpart to
// LangSmithObserver. Safe for concurrent use.
//
// Events are buffered and flushed in batches (one flush per ReplyEnd/Error, or
// when the buffer reaches BatchSize) to minimise HTTP round-trips.
type LangfuseObserver struct {
	client    *LangfuseClient
	userID    string
	sessionID string

	mu     sync.Mutex
	buffer []LangfuseEvent
	// active generations/spans keyed by id, to update end times.
	active map[string]bool
}

// NewLangfuseObserver creates an observer attached to the given client.
func NewLangfuseObserver(client *LangfuseClient, userID, sessionID string) *LangfuseObserver {
	return &LangfuseObserver{
		client:    client,
		userID:    userID,
		sessionID: sessionID,
		active:    map[string]bool{},
	}
}

// Observe blocks and consumes events from the bus until ctx is done, flushing
// any remaining buffer on exit.
func (o *LangfuseObserver) Observe(ctx context.Context, bus *event.Bus) {
	id, ch, _ := bus.Subscribe()
	defer bus.Unsubscribe(id)
	defer o.flush(context.Background())

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

func (o *LangfuseObserver) handle(ctx context.Context, ev event.AgentEvent) {
	switch e := ev.(type) {
	case *event.ReplyStartEvent:
		now := time.Now().UTC().Format(time.RFC3339Nano)
		o.push(NewLangfuseEvent(LangfuseTraceCreate, map[string]any{
			"id":        e.ReplyID(),
			"name":      "agent-reply",
			"userId":    o.userID,
			"sessionId": o.sessionID,
			"metadata":  map[string]any{"agent": e.AgentName},
		}))
		genID := e.ReplyID() + "-gen"
		o.push(NewLangfuseEvent(LangfuseGenerationCreate, map[string]any{
			"id":        genID,
			"traceId":   e.ReplyID(),
			"name":      "llm-call",
			"startTime": now,
			"model":     "",
		}))
		o.markActive(genID)

	case *event.ReplyEndEvent:
		genID := e.ReplyID() + "-gen"
		if o.isActive(genID) {
			o.push(NewLangfuseEvent(LangfuseGenerationCreate, map[string]any{
				"id":      genID,
				"traceId": e.ReplyID(),
				"endTime": time.Now().UTC().Format(time.RFC3339Nano),
			}))
			o.unmarkActive(genID)
		}
		o.flush(ctx)

	case *event.ErrorEvent:
		genID := e.ReplyID() + "-gen"
		if o.isActive(genID) {
			o.push(NewLangfuseEvent(LangfuseGenerationCreate, map[string]any{
				"id":            genID,
				"traceId":       e.ReplyID(),
				"endTime":       time.Now().UTC().Format(time.RFC3339Nano),
				"level":         "ERROR",
				"statusMessage": e.Err,
			}))
			o.unmarkActive(genID)
		}
		o.flush(ctx)

	case *event.ToolCallStartEvent:
		spanID := fmt.Sprintf("%s-span-%d", e.ReplyID(), e.BlockIndex)
		o.push(NewLangfuseEvent(LangfuseSpanCreate, map[string]any{
			"id":        spanID,
			"traceId":   e.ReplyID(),
			"name":      e.ToolName,
			"startTime": time.Now().UTC().Format(time.RFC3339Nano),
			"metadata":  map[string]any{"tool_call_id": e.ToolCallID},
		}))
		o.markActive(spanID)

	case *event.ToolCallEndEvent:
		spanID := fmt.Sprintf("%s-span-%d", e.ReplyID(), e.BlockIndex)
		if o.isActive(spanID) {
			o.push(NewLangfuseEvent(LangfuseSpanCreate, map[string]any{
				"id":      spanID,
				"traceId": e.ReplyID(),
				"endTime": time.Now().UTC().Format(time.RFC3339Nano),
			}))
			o.unmarkActive(spanID)
		}
	}
}

func (o *LangfuseObserver) push(ev LangfuseEvent) {
	o.mu.Lock()
	o.buffer = append(o.buffer, ev)
	o.mu.Unlock()
}

func (o *LangfuseObserver) flush(ctx context.Context) {
	o.mu.Lock()
	if len(o.buffer) == 0 {
		o.mu.Unlock()
		return
	}
	batch := o.buffer
	o.buffer = nil
	o.mu.Unlock()
	_ = o.client.Ingest(ctx, batch) // best-effort; errors don't break the agent
}

func (o *LangfuseObserver) markActive(id string) {
	o.mu.Lock()
	o.active[id] = true
	o.mu.Unlock()
}

func (o *LangfuseObserver) unmarkActive(id string) {
	o.mu.Lock()
	delete(o.active, id)
	o.mu.Unlock()
}

func (o *LangfuseObserver) isActive(id string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.active[id]
}
