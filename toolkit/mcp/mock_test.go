package mcp

import (
	"context"
	"testing"
)

type mockClient struct {
	tools []ToolInfo
}

func (m *mockClient) Connect(ctx context.Context, cfg MCPConfig) error { return nil }

func (m *mockClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	return m.tools, nil
}

func (m *mockClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return map[string]any{"echo": name, "args": args}, nil
}

func (m *mockClient) Close() error { return nil }

func TestManagerTools(t *testing.T) {
	mc := &mockClient{tools: []ToolInfo{{Name: "add", Description: "d", Parameters: map[string]any{}}}}
	mgr := NewManager()
	_ = mgr.Register("s1", mc)
	tools, err := mgr.Tools(context.Background())
	if err != nil || len(tools) != 1 {
		t.Fatal(err, len(tools))
	}
	if tools[0].Name() != "s1/add" {
		t.Fatal(tools[0].Name())
	}
	v, err := tools[0].Execute(context.Background(), map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		t.Fatal("nil result")
	}
}
