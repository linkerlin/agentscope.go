package react

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
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
	a, err := Builder().Name("t").Model(&slowStreamModel{delay: 20 * time.Millisecond, chunks: 100}).Build()
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
