package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit"
)

func TestToolOffloadManager_PushPopResults(t *testing.T) {
	m := NewToolOffloadManager()
	m.PushResult("s1", "hint-a")
	m.PushResult("s1", "hint-b")

	got := m.PopResults("s1")
	if len(got) != 2 || got[0] != "hint-a" || got[1] != "hint-b" {
		t.Fatalf("unexpected hints: %#v", got)
	}
	if m.PopResults("s1") != nil {
		t.Fatal("expected empty after pop")
	}
}

func TestToolOffloadMiddleware_CompletesInTime(t *testing.T) {
	mgr := NewToolOffloadManager().WithTimeout(200 * time.Millisecond)
	mw := NewToolOffloadMiddleware(mgr, "sess1")

	handler := mw.Wrap(func(ctx context.Context, req *toolkit.Request) (*toolkit.Response, error) {
		return &toolkit.Response{Single: tool.NewTextResponse("ok")}, nil
	})

	resp, err := handler(context.Background(), &toolkit.Request{
		Stage:    toolkit.StageExecuteTool,
		ToolName: "fast_tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Single.GetTextContent() != "ok" {
		t.Fatalf("unexpected response: %q", resp.Single.GetTextContent())
	}
}

func TestToolOffloadMiddleware_OffloadsSlowTool(t *testing.T) {
	mgr := NewToolOffloadManager().WithTimeout(30 * time.Millisecond)
	mw := NewToolOffloadMiddleware(mgr, "sess2")

	handler := mw.Wrap(func(ctx context.Context, req *toolkit.Request) (*toolkit.Response, error) {
		time.Sleep(120 * time.Millisecond)
		return &toolkit.Response{Single: tool.NewTextResponse("slow-result")}, nil
	})

	resp, err := handler(context.Background(), &toolkit.Request{
		Stage:    toolkit.StageExecuteTool,
		ToolName: "slow_tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Single.GetTextContent() == "slow-result" {
		t.Fatal("expected placeholder, got final result synchronously")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		hints := mgr.PopResults("sess2")
		if len(hints) == 1 {
			if !containsAll(hints[0], "slow_tool", "slow-result") {
				t.Fatalf("unexpected hint: %q", hints[0])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for background result")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestToolOffloadMiddleware_PassThroughBatchStage(t *testing.T) {
	mw := NewToolOffloadMiddleware(NewToolOffloadManager(), "sess3")
	called := false
	handler := mw.Wrap(func(ctx context.Context, req *toolkit.Request) (*toolkit.Response, error) {
		called = true
		return &toolkit.Response{Results: []toolkit.ToolResult{{Name: "x"}}}, nil
	})
	_, err := handler(context.Background(), &toolkit.Request{Stage: toolkit.StageExecute})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected batch stage to pass through")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if p != "" && !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
