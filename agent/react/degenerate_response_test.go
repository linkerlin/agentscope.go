package react

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// emptyContentModel returns assistant messages with no content blocks.
type emptyContentModel struct {
	mockChatModel
}

func (m *emptyContentModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).Build(), nil
}

func (m *emptyContentModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 1)
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

// TestReActAgent_EmptyResponseErrors ensures a degenerate model turn surfaces
// as an error instead of a silent empty reply (PyV2 #1861).
func TestReActAgent_EmptyResponseErrors(t *testing.T) {
	a, err := Builder().Name("t").Model(&emptyContentModel{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if !errors.Is(err, errEmptyModelResponse) {
		t.Fatalf("expected errEmptyModelResponse, got %v", err)
	}
}

// TestReActAgent_EmptyStreamErrors covers the streaming path and asserts an
// ErrorEvent is emitted before the stream ends.
func TestReActAgent_EmptyStreamErrors(t *testing.T) {
	a, err := Builder().Name("t").Model(&emptyContentModel{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := a.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	sawError := false
	for ev := range ch {
		if _, ok := ev.(*event.ErrorEvent); ok {
			sawError = true
		}
	}
	if !sawError {
		t.Fatal("expected ErrorEvent for empty stream")
	}
}

func TestCheckFinalAnswer_NilResponse(t *testing.T) {
	a, err := Builder().Name("t").Model(&mockChatModel{name: "m"}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.checkFinalAnswer(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil model response")
	}
}

// manyChunkStreamModel blocks on an undersized channel until the consumer
// drains it, exposing producer leaks when a consumer exits early.
type manyChunkStreamModel struct {
	mockChatModel
	produced chan struct{}
}

func (m *manyChunkStreamModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 1)
	go func() {
		defer close(m.produced)
		for i := 0; i < 200; i++ {
			ch <- &model.StreamChunk{Delta: "x"}
		}
		ch <- &model.StreamChunk{Done: true}
		close(ch)
	}()
	return ch, nil
}

// TestRunModelInner_DrainsStreamOnEarlyExit verifies the consumer abandons the
// stream safely: the producer must still finish instead of leaking a blocked
// goroutine and an unclosed HTTP body (E5d).
func TestRunModelInner_DrainsStreamOnEarlyExit(t *testing.T) {
	m := &manyChunkStreamModel{produced: make(chan struct{})}
	sh := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		if ev.EventType() == hook.EventReasoningChunk {
			return nil, hook.ErrInterrupted
		}
		return nil, nil
	})
	a, err := Builder().Name("t").Model(m).StreamHooks(sh).Build()
	if err != nil {
		t.Fatal(err)
	}
	// The hook interrupts the loop after the first chunk; the background
	// drain must let the producer complete.
	_, _ = a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	select {
	case <-m.produced:
	case <-time.After(3 * time.Second):
		t.Fatal("producer goroutine leaked: stream was not drained on early exit")
	}
}

// TestConsecutiveFailureBreaker_HintNeeded verifies the nudge fires one step
// before the breaker trips and goes quiet afterwards (PyV2 tool_retries_hint).
func TestConsecutiveFailureBreaker_HintNeeded(t *testing.T) {
	b := newConsecutiveFailureBreaker(3)
	fail := []failureSignal{{ToolName: "x", IsError: true, ErrText: "boom"}}

	b.update(fail)
	if name, _, _ := b.hintNeeded(); name != "" {
		t.Fatalf("single failure must stay quiet, got hint for %q", name)
	}
	b.update(fail)
	name, count, reason := b.hintNeeded()
	if name != "x" || count != 2 || reason != "boom" {
		t.Fatalf("expected hint for x/2/boom, got %q/%d/%q", name, count, reason)
	}
	if tripped, _, _ := b.update(fail); tripped != "x" {
		t.Fatalf("expected breaker to trip, got %q", tripped)
	}
	if name, _, _ := b.hintNeeded(); name != "" {
		t.Fatalf("hint must go quiet after the breaker tripped, got %q", name)
	}

	// A success resets the count.
	b2 := newConsecutiveFailureBreaker(3)
	b2.update(fail)
	b2.update(fail)
	b2.update([]failureSignal{{ToolName: "x"}})
	if name, _, _ := b2.hintNeeded(); name != "" {
		t.Fatalf("success should reset the count, got hint for %q", name)
	}
}

func TestToolRetriesHintText(t *testing.T) {
	text := toolRetriesHintText("search", 2, "timeout")
	for _, want := range []string{"search", "2 times", "timeout", "<system-reminder>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in hint: %q", want, text)
		}
	}
}

// TestReActAgent_ToolRetriesHintInjected verifies a repeatedly failing tool
// gets a <system-reminder> nudge fed back to the model before the breaker
// trips.
func TestReActAgent_ToolRetriesHintInjected(t *testing.T) {
	failTool := tool.NewFunctionTool("flaky", "always fails", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return nil, errors.New("boom")
	})
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "flaky", map[string]any{}),
	).Build()
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("giving up").Build()
	m := &recordingToolModel{responses: []*message.Msg{toolCallMsg, toolCallMsg, toolCallMsg, finalMsg}}
	a, err := Builder().Name("t").Model(m).Tools(failTool).Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()); err != nil {
		t.Fatal(err)
	}
	if len(m.seen) < 3 {
		t.Fatalf("expected at least 3 model calls, got %d", len(m.seen))
	}
	found := false
	for _, b := range m.seen[2] {
		if b.Role == message.RoleSystem && strings.Contains(b.GetTextContent(), "has failed 2 times") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected tool-retries hint in the third model call history")
	}
}

// recordingToolModel replays scripted responses while recording inputs.
type recordingToolModel struct {
	responses []*message.Msg
	calls     int
	seen      [][]*message.Msg
}

func (m *recordingToolModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.seen = append(m.seen, messages)
	resp := m.responses[m.calls%len(m.responses)]
	m.calls++
	return resp, nil
}

func (m *recordingToolModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	msg, err := m.Chat(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	ch := make(chan *model.StreamChunk, 4)
	if text := msg.GetTextContent(); text != "" {
		ch <- &model.StreamChunk{Delta: text}
	}
	if content := msg.Content; len(content) > 0 {
		ch <- &model.StreamChunk{Done: true, Content: content}
	} else {
		ch <- &model.StreamChunk{Done: true}
	}
	close(ch)
	return ch, nil
}

func (m *recordingToolModel) ModelName() string { return "recording" }

// slowStreamModel emits chunks with a delay so an interrupt lands mid-stream.
type slowStreamModel struct {
	mockChatModel
	delay  time.Duration
	chunks int
}

func (m *slowStreamModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 64)
	go func() {
		defer close(ch)
		for i := 0; i < m.chunks; i++ {
			select {
			case ch <- &model.StreamChunk{Delta: "x"}:
			case <-ctx.Done():
				return
			}
			time.Sleep(m.delay)
		}
		ch <- &model.StreamChunk{Done: true}
	}()
	return ch, nil
}

// TestReActAgent_InterruptStopsStream verifies an interrupt requested
// mid-stream stops consumption promptly and takes the recovery path instead
// of reading the whole stream first (E4b).
func TestReActAgent_InterruptStopsStream(t *testing.T) {
	// A no-op stream hook forces the V1 path onto ChatStream so the interrupt
	// lands mid-stream instead of racing a synchronous Chat return.
	noopHook := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		return nil, nil
	})
	a, err := Builder().Name("t").
		Model(&slowStreamModel{delay: 20 * time.Millisecond, chunks: 100}).
		StreamHooks(noopHook).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		resp *message.Msg
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
		done <- result{resp, err}
	}()
	// Let the stream start, then interrupt mid-stream.
	time.Sleep(100 * time.Millisecond)
	a.Interrupt()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("expected recovery message, got error: %v", r.err)
		}
		if r.resp == nil || r.resp.GetTextContent() == "" {
			t.Fatalf("expected non-empty recovery message, got %+v", r.resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("interrupt did not stop stream consumption promptly")
	}
}
