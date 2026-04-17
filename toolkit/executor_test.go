package toolkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

type panicTool struct{}

func (panicTool) Name() string        { return "panic" }
func (panicTool) Description() string { return "always panics" }
func (panicTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: "panic", Description: "always panics"}
}
func (panicTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	panic("intentional panic")
}

type slowTool struct{}

func (slowTool) Name() string        { return "slow" }
func (slowTool) Description() string { return "slow tool" }
func (slowTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: "slow", Description: "slow tool"}
}
func (slowTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return tool.NewTextResponse("done"), nil
	}
}

type failTool struct{}

func (failTool) Name() string        { return "fail" }
func (failTool) Description() string { return "always fails" }
func (failTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: "fail", Description: "always fails"}
}
func (failTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return nil, errors.New("fail")
}

func newTestRegistry(tools ...tool.Tool) *Registry {
	r := NewRegistry()
	for _, t := range tools {
		_ = r.Register(t)
	}
	return r
}

func TestToolExecutor_PanicRecovery(t *testing.T) {
	reg := newTestRegistry(panicTool{})
	exec := NewToolExecutor(DefaultExecutionConfig())

	results, err := exec.Execute(context.Background(), reg, []ToolCall{{ID: "1", Name: "panic"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected error for panicking tool")
	}
	if results[0].Err.Error() != "tool panic recovered: intentional panic" {
		t.Fatalf("unexpected error message: %v", results[0].Err)
	}
}

func TestToolExecutor_Timeout(t *testing.T) {
	reg := newTestRegistry(slowTool{})
	exec := NewToolExecutor(ExecutionConfig{Timeout: 50 * time.Millisecond, MaxRetries: 1})

	results, err := exec.Execute(context.Background(), reg, []ToolCall{{ID: "1", Name: "slow"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(results[0].Err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got: %v", results[0].Err)
	}
}

func TestToolExecutor_Retry(t *testing.T) {
	reg := newTestRegistry(failTool{})
	exec := NewToolExecutor(ExecutionConfig{MaxRetries: 3})

	results, err := exec.Execute(context.Background(), reg, []ToolCall{{ID: "1", Name: "fail"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err == nil {
		t.Fatal("expected error after retries")
	}
}

func TestToolExecutor_Parallel(t *testing.T) {
	reg := newTestRegistry(slowTool{})
	exec := NewToolExecutor(ExecutionConfig{Timeout: 200 * time.Millisecond, Parallel: true, MaxParallel: 2})

	calls := []ToolCall{
		{ID: "1", Name: "slow"},
		{ID: "2", Name: "slow"},
	}
	start := time.Now()
	results, err := exec.Execute(context.Background(), reg, calls)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// If truly parallel, should take ~200ms, not ~400ms
	if elapsed > 350*time.Millisecond {
		t.Fatalf("parallel execution too slow: %v", elapsed)
	}
}


func TestToolExecutor_ExecuteTool(t *testing.T) {
	reg := newTestRegistry(tool.NewFunctionTool("echo", "", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	}))
	exec := NewToolExecutor(DefaultExecutionConfig())
	resp, err := exec.ExecuteTool(context.Background(), reg, "echo", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
}

func TestToolExecutor_ExecuteTool_NotFound(t *testing.T) {
	reg := NewRegistry()
	exec := NewToolExecutor(DefaultExecutionConfig())
	_, err := exec.ExecuteTool(context.Background(), reg, "missing", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
