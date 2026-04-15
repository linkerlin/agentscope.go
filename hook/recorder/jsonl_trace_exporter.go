package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/observability"
)

// JsonlTraceExporter writes agent execution traces to a local JSONL file
// via the StreamHook event system. It is the Go equivalent of AgentScope-Java's
// built-in JSONL trace exporter.
type JsonlTraceExporter struct {
	path    string
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex

	includeReasoningChunks bool
	includeActingChunks    bool
	includeSummary         bool
	includeSummaryChunks   bool
	failFast               bool

	runID  string
	turnID int
	stepID int
}

// Builder constructs a JsonlTraceExporter with fluent options.
type Builder struct {
	path                   string
	includeReasoningChunks bool
	includeActingChunks    bool
	includeSummary         bool
	includeSummaryChunks   bool
	failFast               bool
}

// NewBuilder creates a builder for the given output file path.
func NewBuilder(path string) *Builder {
	return &Builder{path: path}
}

// IncludeReasoningChunks controls whether ReasoningChunk events are recorded.
func (b *Builder) IncludeReasoningChunks(v bool) *Builder {
	b.includeReasoningChunks = v
	return b
}

// IncludeActingChunks controls whether ActingChunk events are recorded.
func (b *Builder) IncludeActingChunks(v bool) *Builder {
	b.includeActingChunks = v
	return b
}

// IncludeSummary controls whether summary-related events are recorded.
func (b *Builder) IncludeSummary(v bool) *Builder {
	b.includeSummary = v
	return b
}

// IncludeSummaryChunks controls whether summary-chunk events are recorded.
func (b *Builder) IncludeSummaryChunks(v bool) *Builder {
	b.includeSummaryChunks = v
	return b
}

// FailFast makes IO/serialization errors propagate up and interrupt the agent.
// When false (default) the exporter operates in best-effort mode.
func (b *Builder) FailFast(v bool) *Builder {
	b.failFast = v
	return b
}

// Build opens the output file and returns the configured exporter.
func (b *Builder) Build() (*JsonlTraceExporter, error) {
	dir := filepath.Dir(b.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("jsonl exporter: create directory: %w", err)
		}
	}
	f, err := os.OpenFile(b.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("jsonl exporter: open file: %w", err)
	}
	return &JsonlTraceExporter{
		path:                   b.path,
		file:                   f,
		encoder:                json.NewEncoder(f),
		includeReasoningChunks: b.includeReasoningChunks,
		includeActingChunks:    b.includeActingChunks,
		includeSummary:         b.includeSummary,
		includeSummaryChunks:   b.includeSummaryChunks,
		failFast:               b.failFast,
		runID:                  uuid.New().String(),
	}, nil
}

// OnStreamEvent implements hook.StreamHook.
func (e *JsonlTraceExporter) OnStreamEvent(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
	if err := e.handleEvent(ctx, ev); err != nil {
		if e.failFast {
			return nil, err
		}
		// best-effort: swallow the error so agent execution continues
	}
	return nil, nil
}

func (e *JsonlTraceExporter) handleEvent(ctx context.Context, ev hook.Event) error {
	// update counters and filter unwanted events
	switch ev.EventType() {
	case hook.EventPreCall:
		e.mu.Lock()
		e.turnID++
		e.stepID = 0
		e.mu.Unlock()
	case hook.EventPreReasoning:
		e.mu.Lock()
		e.stepID++
		e.mu.Unlock()
	case hook.EventReasoningChunk:
		if !e.includeReasoningChunks {
			return nil
		}
	case hook.EventActingChunk:
		if !e.includeActingChunks {
			return nil
		}
	}

	rec := e.buildRecord(ctx, ev)

	e.mu.Lock()
	defer e.mu.Unlock()
	return e.encoder.Encode(rec)
}

func (e *JsonlTraceExporter) buildRecord(ctx context.Context, ev hook.Event) map[string]any {
	e.mu.Lock()
	turn := e.turnID
	step := e.stepID
	run := e.runID
	e.mu.Unlock()

	rec := map[string]any{
		"ts":         ev.Timestamp().Format(time.RFC3339Nano),
		"event_type": string(ev.EventType()),
		"agent_name": ev.AgentName(),
		"run_id":     run,
		"turn_id":    turn,
		"step_id":    step,
	}

	if tc := observability.TraceContextFromContext(ctx); tc.IsValid() {
		rec["trace_id"] = tc.TraceID
		rec["span_id"] = tc.SpanID
	}

	switch v := ev.(type) {
	case *hook.PreReasoningEvent:
		rec["input_messages"] = msgsToMaps(v.Messages)
		rec["model_name"] = v.ModelName
	case *hook.PostReasoningEvent:
		rec["messages"] = msgsToMaps(v.Messages)
		rec["response"] = msgToMap(v.Response)
	case *hook.ReasoningChunkEvent:
		rec["chunk"] = v.Chunk
		rec["messages"] = msgsToMaps(v.Messages)
	case *hook.PreActingEvent:
		rec["tool_name"] = v.ToolName
		rec["tool_input"] = v.ToolInput
		rec["messages"] = msgsToMaps(v.Messages)
	case *hook.PostActingEvent:
		rec["tool_name"] = v.ToolName
		rec["tool_input"] = v.ToolInput
		rec["result"] = v.Result
		if v.Err != nil {
			rec["error"] = v.Err.Error()
		}
		rec["messages"] = msgsToMaps(v.Messages)
	case *hook.ActingChunkEvent:
		rec["tool_name"] = v.ToolName
		rec["chunk"] = v.Chunk
	case *hook.ErrorEvent:
		if v.Err != nil {
			rec["error_class"] = fmt.Sprintf("%T", v.Err)
			rec["error_message"] = v.Err.Error()
		}
	}

	return rec
}

// Close flushes and closes the underlying file.
func (e *JsonlTraceExporter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

func msgsToMaps(msgs []*message.Msg) []map[string]any {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		if mp := msgToMap(m); mp != nil {
			out = append(out, mp)
		}
	}
	return out
}

func msgToMap(m *message.Msg) map[string]any {
	if m == nil {
		return nil
	}
	data, _ := json.Marshal(m)
	var mp map[string]any
	_ = json.Unmarshal(data, &mp)
	return mp
}

var _ hook.StreamHook = (*JsonlTraceExporter)(nil)
