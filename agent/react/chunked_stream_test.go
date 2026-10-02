package react

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// streamChunkedTool streams its chunks with a real delay so the ordering
// assertions are about the pipeline, not scheduling luck.
type streamChunkedTool struct {
	chunks []string
	delay  time.Duration
}

func (t *streamChunkedTool) Name() string        { return "streamer" }
func (t *streamChunkedTool) Description() string { return "streams" }
func (t *streamChunkedTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: "streamer", Description: "streams", Parameters: map[string]any{}}
}
func (t *streamChunkedTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return t.ExecuteChunked(ctx, input, func(string) {})
}
func (t *streamChunkedTool) ExecuteChunked(ctx context.Context, input map[string]any, emit func(chunk string)) (*tool.Response, error) {
	for _, c := range t.chunks {
		select {
		case <-time.After(t.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		emit(c)
	}
	return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock("final")}}, nil
}

var _ tool.ChunkedTool = (*streamChunkedTool)(nil)

// toolCallThenTextModel issues one tool call, then answers with text.
type toolCallThenTextModel struct {
	step int
}

func (m *toolCallThenTextModel) ModelName() string { return "tc" }
func (m *toolCallThenTextModel) reply() (*message.Msg, error) {
	m.step++
	if m.step == 1 {
		msg := message.NewMsg().Role(message.RoleAssistant).Build()
		msg.Content = append(msg.Content, message.NewToolUseBlock("c1", m.wantedTool(), map[string]any{}))
		return msg, nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("done").Build(), nil
}

func (m *toolCallThenTextModel) wantedTool() string { return "streamer" }

func (m *toolCallThenTextModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return m.reply()
}

func (m *toolCallThenTextModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	msg, err := m.reply()
	if err != nil {
		return nil, err
	}
	ch := make(chan *model.StreamChunk, 2)
	if msg.GetTextContent() != "" {
		ch <- &model.StreamChunk{Delta: msg.GetTextContent()}
	}
	ch <- &model.StreamChunk{Content: msg.Content, Done: true}
	return ch, nil
}

// plainToolModel targets the plain tool for the unchanged-behaviour case.
type plainToolModel struct{ toolCallThenTextModel }

func (m *plainToolModel) wantedTool() string { return "plain" }

type plainStreamTool struct{}

func (t *plainStreamTool) Name() string        { return "plain" }
func (t *plainStreamTool) Description() string { return "plain" }
func (t *plainStreamTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: "plain", Description: "plain", Parameters: map[string]any{}}
}
func (t *plainStreamTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock("ok")}}, nil
}

// TestReplyStreamToolChunks is the 20.6 acceptance at the agent layer: the
// chunked tool's first delta is emitted BEFORE the tool call end (visible
// mid-execution), the delta order is the emit order, and the result flow
// afterwards is unchanged.
func TestReplyStreamToolChunks(t *testing.T) {
	streamer := &streamChunkedTool{chunks: []string{"alpha", "beta", "gamma"}, delay: 40 * time.Millisecond}
	a, err := Builder().
		Name("chunked").
		Model(&toolCallThenTextModel{}).
		Memory(memory.NewInMemoryMemory()).
		Tools(streamer).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent("run it").Build()

	ch, err := a.ReplyStream(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	var seq []event.AgentEvent
	for ev := range ch {
		seq = append(seq, ev)
		if _, ok := ev.(*event.ReplyEndEvent); ok {
			break
		}
	}

	// Locate the tool-call events for call c1.
	var callStart, firstDelta, callEnd int = -1, -1, -1
	var deltas []string
	for i, ev := range seq {
		switch e := ev.(type) {
		case *event.ToolCallStartEvent:
			if e.ToolCallID == "c1" && callStart < 0 {
				callStart = i
			}
		case *event.ToolCallDeltaEvent:
			if e.ToolCallID == "c1" {
				deltas = append(deltas, e.Delta)
				if firstDelta < 0 {
					firstDelta = i
				}
			}
		case *event.ToolCallEndEvent:
			if e.ToolCallID == "c1" && callEnd < 0 {
				callEnd = i
			}
		}
	}
	if callStart < 0 || callEnd < 0 {
		t.Fatalf("missing tool call events: start=%d end=%d", callStart, callEnd)
	}
	if len(deltas) != 3 || strings.Join(deltas, "") != "alphabetagamma" {
		t.Fatalf("deltas wrong: %v", deltas)
	}
	if !(callStart < firstDelta && firstDelta < callEnd) {
		t.Fatalf("first delta must land between start and end: start=%d delta=%d end=%d", callStart, firstDelta, callEnd)
	}
}

// TestReplyStreamPlainToolNoDelta: a plain tool keeps the immediate
// Start+End pair with no deltas between them (pre-20.6 behaviour).
func TestReplyStreamPlainToolNoDelta(t *testing.T) {
	a, err := Builder().
		Name("plain").
		Model(&plainToolModel{}).
		Memory(memory.NewInMemoryMemory()).
		Tools(&plainStreamTool{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent("run it").Build()
	ch2, err2 := a.ReplyStream(context.Background(), msg)
	if err2 != nil {
		t.Fatal(err2)
	}
	var seq []event.AgentEvent
	for ev := range ch2 {
		seq = append(seq, ev)
		if _, ok := ev.(*event.ReplyEndEvent); ok {
			break
		}
	}
	startIdx, endIdx := -1, -1
	for i, ev := range seq {
		if s, ok := ev.(*event.ToolCallStartEvent); ok && s.ToolCallID == "c1" {
			startIdx = i
		}
		if e, ok := ev.(*event.ToolCallEndEvent); ok && e.ToolCallID == "c1" {
			endIdx = i
		}
	}
	if startIdx < 0 || endIdx < 0 {
		t.Fatalf("missing events: %d %d", startIdx, endIdx)
	}
	if endIdx != startIdx+1 {
		t.Fatalf("plain tool must keep adjacent Start+End, got %d..%d", startIdx, endIdx)
	}
}
