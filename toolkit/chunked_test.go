package toolkit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// slowChunkedTool is the 20.6 acceptance instrument: a tool whose execution
// streams chunks with real delays, so "first chunk before completion" is
// observable on the wall clock.
type slowChunkedTool struct {
	name       string
	chunks     []string
	delay      time.Duration
	errAfter   int  // >0: return an error after this many chunks
	cancelTest bool // wait for ctx cancellation instead of finishing
	resp       string
}

func (t *slowChunkedTool) Name() string        { return t.name }
func (t *slowChunkedTool) Description() string { return "slow chunked" }
func (t *slowChunkedTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: t.name, Description: "slow chunked", Parameters: map[string]any{}}
}
func (t *slowChunkedTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return t.ExecuteChunked(ctx, input, func(string) {})
}
func (t *slowChunkedTool) ExecuteChunked(ctx context.Context, input map[string]any, emit func(chunk string)) (*tool.Response, error) {
	for i, c := range t.chunks {
		if t.cancelTest {
			select {
			case <-time.After(t.delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else if t.delay > 0 {
			select {
			case <-time.After(t.delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		emit(c)
		if t.errAfter > 0 && i+1 == t.errAfter {
			return nil, errors.New("chunked tool exploded mid-stream")
		}
	}
	return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock(t.resp)}}, nil
}

// collector records chunks with arrival times.
type collector struct {
	mu     sync.Mutex
	chunks []string
	at     []time.Time
}

func (c *collector) emit(chunk string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chunks = append(c.chunks, chunk)
	c.at = append(c.at, time.Now())
}

func (c *collector) snapshot() ([]string, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	first := time.Time{}
	if len(c.at) > 0 {
		first = c.at[0]
	}
	return append([]string(nil), c.chunks...), first
}

// TestExecuteToolChunked_FirstChunkBeforeCompletion is the acceptance's
// first clause: with a tool streaming 3 chunks 60ms apart (~180ms total),
// the first chunk is observed well before the execution finishes.
func TestExecuteToolChunked_FirstChunkBeforeCompletion(t *testing.T) {
	tk := NewToolkit()
	toolSlow := &slowChunkedTool{name: "slow", chunks: []string{"a", "b", "c"}, delay: 60 * time.Millisecond, resp: "done"}
	tk.Register(toolSlow)

	c := &collector{}
	start := time.Now()
	resp, err := tk.ExecuteToolChunked(context.Background(), "slow", nil, c.emit)
	elapsed := time.Since(start)
	if err != nil || resp == nil {
		t.Fatalf("exec: %v", err)
	}
	chunks, firstAt := c.snapshot()
	if len(chunks) != 3 {
		t.Fatalf("chunks: %v", chunks)
	}
	if want := []string{"a", "b", "c"}; strings.Join(chunks, "") != strings.Join(want, "") {
		t.Fatalf("chunk order lost: %v", chunks)
	}
	// First chunk arrived within ~one delay of the start (not after all 3).
	if firstAt.Sub(start) > 2*60*time.Millisecond {
		t.Fatalf("first chunk too late: %v (total %v)", firstAt.Sub(start), elapsed)
	}
	if elapsed < 3*60*time.Millisecond {
		t.Fatalf("tool finished impossibly fast: %v", elapsed)
	}
}

// TestExecuteToolChunked_PlainToolUnchanged: non-chunked tools run exactly
// like ExecuteTool; emit is simply never called.
func TestExecuteToolChunked_PlainToolUnchanged(t *testing.T) {
	tk := NewToolkit()
	tk.Register(&mockPlainTool{name: "plain", out: "ok"})
	c := &collector{}
	resp, err := tk.ExecuteToolChunked(context.Background(), "plain", nil, c.emit)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if chunks, _ := c.snapshot(); len(chunks) != 0 {
		t.Fatalf("plain tool must not emit chunks: %v", chunks)
	}
}

// TestExecuteToolChunked_MidStreamError: a tool failing after some chunks
// keeps the already-emitted chunks delivered and surfaces the error.
func TestExecuteToolChunked_MidStreamError(t *testing.T) {
	tk := NewToolkit()
	tk.Register(&slowChunkedTool{name: "boom", chunks: []string{"x", "y", "z"}, errAfter: 2, resp: "never"})
	c := &collector{}
	_, err := tk.ExecuteToolChunked(context.Background(), "boom", nil, c.emit)
	if err == nil || !strings.Contains(err.Error(), "exploded") {
		t.Fatalf("expected mid-stream error, got %v", err)
	}
	if chunks, _ := c.snapshot(); len(chunks) != 2 {
		t.Fatalf("chunks before the error must stay delivered: %v", chunks)
	}
}

// TestExecuteToolChunked_Cancellation: cancelling the context stops the
// tool; delivered chunks stay, the error is the context's.
func TestExecuteToolChunked_Cancellation(t *testing.T) {
	tk := NewToolkit()
	tk.Register(&slowChunkedTool{name: "slowcancel", chunks: []string{"1", "2", "3", "4"}, delay: 40 * time.Millisecond, cancelTest: true, resp: "done"})
	c := &collector{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := tk.ExecuteToolChunked(ctx, "slowcancel", nil, c.emit)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected ctx error, got %v", err)
	}
	chunks, _ := c.snapshot()
	if len(chunks) < 1 {
		t.Fatal("at least one chunk should have landed before cancellation")
	}
}

// mockPlainTool is a minimal non-chunked tool.
type mockPlainTool struct {
	name string
	out  string
}

func (m *mockPlainTool) Name() string        { return m.name }
func (m *mockPlainTool) Description() string { return "plain" }
func (m *mockPlainTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: m.name, Description: "plain", Parameters: map[string]any{}}
}
func (m *mockPlainTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock(m.out)}}, nil
}
