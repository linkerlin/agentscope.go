package toolkit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/workspace"
)

func TestChain(t *testing.T) {
	var order []string
	mw1 := &testMiddleware{name: "mw1", order: &order}
	mw2 := &testMiddleware{name: "mw2", order: &order}

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		order = append(order, "handler")
		return &Response{}, nil
	}

	chained := chain(handler, mw1, mw2)
	_, err := chained(context.Background(), &Request{})
	if err != nil {
		t.Fatal(err)
	}

	// Middleware wraps from outer to inner: mw1 -> mw2 -> handler
	expected := []string{"mw1-pre", "mw2-pre", "handler", "mw2-post", "mw1-post"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected %v, got %v", expected, order)
		}
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var logs []string
	logf := func(f string, a ...interface{}) {
		logs = append(logs, fmt.Sprintf(f, a...))
	}
	mw := NewLoggingMiddleware(logf)

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{Results: []ToolResult{{ID: "1"}}}, nil
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "t1"}}})
	if err != nil {
		t.Fatal(err)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %v", len(logs), logs)
	}
	if logs[0] != "[toolkit] execute start: 1 calls" {
		t.Fatalf("unexpected first log: %s", logs[0])
	}
}

func TestMetricsMiddleware(t *testing.T) {
	var captured struct {
		stage MiddlewareStage
		count int
		err   error
	}
	mw := NewMetricsMiddleware(func(s MiddlewareStage, c int, d time.Duration, e error) {
		captured.stage = s
		captured.count = c
		captured.err = e
	})

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{}, errors.New("boom")
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "t1"}, {Name: "t2"}}})
	if err == nil {
		t.Fatal("expected error")
	}

	if captured.stage != StageExecute {
		t.Fatalf("expected stage execute, got %s", captured.stage)
	}
	if captured.count != 2 {
		t.Fatalf("expected count 2, got %d", captured.count)
	}
	if captured.err == nil {
		t.Fatal("expected captured error")
	}
}

func TestPermissionMiddleware(t *testing.T) {
	mw := NewPermissionMiddleware(func(ctx context.Context, calls []ToolCall) error {
		for _, c := range calls {
			if c.Name == "blocked" {
				return errors.New("blocked tool")
			}
		}
		return nil
	})

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{}, nil
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "allowed"}}})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	_, err = chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "blocked"}}})
	if err == nil {
		t.Fatal("expected permission denied")
	}
}

func TestToolkitWithMiddleware(t *testing.T) {
	tk := NewToolkit()

	// Register a dummy tool.
	dummy := &dummyTool{name: "echo"}
	if err := tk.Register(dummy); err != nil {
		t.Fatal(err)
	}

	var order []string
	tk.Use(&testMiddleware{name: "mw1", order: &order})

	_, err := tk.ExecuteTool(context.Background(), "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("expected middleware chain to run, got order: %v", order)
	}
}

type testMiddleware struct {
	name  string
	order *[]string
}

func (m *testMiddleware) Wrap(next Handler) Handler {
	return func(ctx context.Context, req *Request) (*Response, error) {
		*m.order = append(*m.order, m.name+"-pre")
		resp, err := next(ctx, req)
		*m.order = append(*m.order, m.name+"-post")
		return resp, err
	}
}

type dummyTool struct {
	name string
}

func (d *dummyTool) Name() string        { return d.name }
func (d *dummyTool) Description() string { return "dummy tool" }
func (d *dummyTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: d.name, Description: "dummy"}
}
func (d *dummyTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock("ok")}}, nil
}

func TestTracingMiddleware(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("test")

	mw := NewTracingMiddleware(tracer)
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{Results: []ToolResult{{Name: "t1"}}}, nil
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "t1"}}})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestTracingMiddlewareError(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("test")

	mw := NewTracingMiddleware(tracer)
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return nil, errors.New("boom")
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecuteTool, ToolName: "t1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOffloadMiddleware(t *testing.T) {
	ws := workspace.NewLocalWorkspace("offload-test", "")
	mw := NewOffloadMiddleware(ws, "/tmp/offload")

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{Results: []ToolResult{{Name: "echo", Response: &tool.Response{Content: []message.ContentBlock{message.NewTextBlock("ok")}}}}}, nil
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "echo"}}})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestOffloadMiddlewareNilWorkspace(t *testing.T) {
	mw := NewOffloadMiddleware(nil, "/tmp/offload")

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{Results: []ToolResult{{Name: "echo"}}}, nil
	}

	chained := mw.Wrap(handler)
	_, err := chained(context.Background(), &Request{Stage: StageExecute, ToolCalls: []ToolCall{{Name: "echo"}}})
	if err != nil {
		t.Fatalf("expected success with nil workspace, got %v", err)
	}
}
