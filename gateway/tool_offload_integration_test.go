package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit"
)

func TestSessionIDFromContext(t *testing.T) {
	ctx := ContextWithSessionID(context.Background(), "sess-abc")
	if SessionIDFromContext(ctx) != "sess-abc" {
		t.Fatal("expected session id in context")
	}
}

func TestToolOffload_ContextSessionHintInjection(t *testing.T) {
	mgr := NewToolOffloadManager().WithTimeout(25 * time.Millisecond)
	mw := NewToolOffloadMiddleware(mgr, "")

	handler := mw.Wrap(func(ctx context.Context, req *toolkit.Request) (*toolkit.Response, error) {
		time.Sleep(80 * time.Millisecond)
		return &toolkit.Response{Single: tool.NewTextResponse("slow-done")}, nil
	})

	ctx := ContextWithSessionID(context.Background(), "sess-offload")
	resp, err := handler(ctx, &toolkit.Request{
		Stage:    toolkit.StageExecuteTool,
		ToolName: "slow_demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Single.GetTextContent(), "background") {
		t.Fatalf("expected offload placeholder, got %q", resp.Single.GetTextContent())
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		injected := injectOffloadHints(&Server{toolOffload: mgr}, "sess-offload", "continue")
		if strings.Contains(injected, "slow-done") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for offload hint injection")
}

func TestToolOffloadMiddleware_ContextSessionID(t *testing.T) {
	mgr := NewToolOffloadManager().WithTimeout(25 * time.Millisecond)
	mw := NewToolOffloadMiddleware(mgr, "")

	handler := mw.Wrap(func(ctx context.Context, req *toolkit.Request) (*toolkit.Response, error) {
		time.Sleep(80 * time.Millisecond)
		return &toolkit.Response{Single: tool.NewTextResponse("ok")}, nil
	})

	ctx := ContextWithSessionID(context.Background(), "ctx-sess")
	_, _ = handler(ctx, &toolkit.Request{Stage: toolkit.StageExecuteTool, ToolName: "t1"})

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if len(mgr.PopResults("ctx-sess")) == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("hint not stored under context session id")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
