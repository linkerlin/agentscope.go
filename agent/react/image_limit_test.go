package react

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

func imgMsg(n int) *message.Msg {
	blocks := make([]message.ContentBlock, 0, n+1)
	blocks = append(blocks, message.NewTextBlock("see:"))
	for i := 0; i < n; i++ {
		blocks = append(blocks, message.NewImageBlock("", "aGVsbG8=", "image/png"))
	}
	return message.NewMsg().Role(message.RoleUser).Content(blocks...).Build()
}

func countImages(msgs []*message.Msg) (images, placeholders int) {
	for _, m := range msgs {
		if m == nil {
			continue
		}
		for _, b := range m.Content {
			switch v := b.(type) {
			case *message.ImageBlock:
				images++
			case *message.TextBlock:
				if strings.Contains(v.Text, "context image limit") {
					placeholders++
				}
			}
		}
	}
	return images, placeholders
}

func TestLimitImages(t *testing.T) {
	src := []*message.Msg{imgMsg(3)}
	out := limitImages(src, 2)
	images, placeholders := countImages(out)
	if images != 2 || placeholders != 1 {
		t.Fatalf("expected 2 images + 1 placeholder, got %d + %d", images, placeholders)
	}
	// The stored message must be untouched.
	if images, _ := countImages(src); images != 3 {
		t.Fatalf("source message mutated: %d images", images)
	}
	// Zero means unlimited.
	if out := limitImages(src, 0); out[0] != src[0] {
		t.Fatal("unlimited must return the input slice as-is")
	}
	// Text-only messages pass through untouched.
	plain := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	if out := limitImages([]*message.Msg{plain}, 1); out[0] != plain {
		t.Fatal("text-only message must pass through untouched")
	}
}

func TestLimitImages_DataBlock(t *testing.T) {
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewDataBlock(message.TypeImage, &message.Source{Type: message.SourceTypeBase64, MediaType: "image/png", Data: "aGVsbG8="}),
		message.NewDataBlock(message.TypeImage, &message.Source{Type: message.SourceTypeBase64, MediaType: "image/png", Data: "aGVsbG8="}),
	).Build()
	out := limitImages([]*message.Msg{msg}, 1)
	images, placeholders := 0, 0
	for _, b := range out[0].Content {
		if db, ok := b.(*message.DataBlock); ok && db.BlockType() == message.TypeImage {
			images++
		}
		if tb, ok := b.(*message.TextBlock); ok && strings.Contains(tb.Text, "context image limit") {
			placeholders++
		}
	}
	if images != 1 || placeholders != 1 {
		t.Fatalf("expected 1 image + 1 placeholder, got %d + %d", images, placeholders)
	}
}

// TestReActAgent_RaiseCancelledOnInterrupt verifies user interrupts surface
// as context.Canceled when configured (PyV2 interruption_raise_cancelled_error).
func TestReActAgent_RaiseCancelledOnInterrupt(t *testing.T) {
	// A no-op stream hook forces the V1 path onto ChatStream so the interrupt
	// lands mid-stream instead of racing a synchronous Chat return.
	noopHook := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		return nil, nil
	})
	a, err := Builder().Name("t").
		Model(&slowStreamModel{delay: 20 * time.Millisecond, chunks: 100}).
		StreamHooks(noopHook).
		RaiseCancelledOnInterrupt().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
		done <- result{err}
	}()
	time.Sleep(100 * time.Millisecond)
	a.Interrupt()
	select {
	case r := <-done:
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("interrupt did not stop the turn promptly")
	}
}
