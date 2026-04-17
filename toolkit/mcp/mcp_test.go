package mcp

import (
	"context"
	"errors"
	"testing"
)

func TestManager_RegisterErrors(t *testing.T) {
	mgr := NewManager()
	mc := &mockClient{}
	if err := mgr.Register("", mc); err == nil {
		t.Fatal("expected error for empty label")
	}
	if err := mgr.Register("s1", nil); err == nil {
		t.Fatal("expected error for nil client")
	}
	_ = mgr.Register("s1", mc)
	if err := mgr.Register("s1", mc); err == nil {
		t.Fatal("expected error for duplicate label")
	}
}

func TestManager_Tools_ListToolsError(t *testing.T) {
	mc := &mockClient{}
	mgr := NewManager()
	_ = mgr.Register("s1", mc)
	// mockClient returns empty tools without error; use a failing client
	failClient := &mockClientFailList{}
	_ = mgr.Register("s2", failClient)
	_, err := mgr.Tools(context.Background())
	if err == nil {
		t.Fatal("expected error from list tools")
	}
}

type mockClientFailList struct{}

func (m *mockClientFailList) Connect(ctx context.Context, cfg MCPConfig) error { return nil }
func (m *mockClientFailList) ListTools(ctx context.Context) ([]ToolInfo, error) {
	return nil, errors.New("list failed")
}
func (m *mockClientFailList) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return nil, nil
}
func (m *mockClientFailList) Close() error { return nil }

func TestToolAdapter_DescriptionAndSpec(t *testing.T) {
	mc := &mockClient{}
	ta := NewToolAdapter("s1", mc, ToolInfo{Name: "add", Description: "adds numbers", Parameters: map[string]any{"type": "object"}})
	if ta.Description() != "adds numbers" {
		t.Fatalf("unexpected description: %s", ta.Description())
	}
	spec := ta.Spec()
	if spec.Name != "s1/add" || spec.Description != "adds numbers" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
}

func TestToolAdapter_ExecuteError(t *testing.T) {
	failClient := &mockClientFailCall{}
	ta := NewToolAdapter("s1", failClient, ToolInfo{Name: "add"})
	_, err := ta.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

type mockClientFailCall struct{}

func (m *mockClientFailCall) Connect(ctx context.Context, cfg MCPConfig) error { return nil }
func (m *mockClientFailCall) ListTools(ctx context.Context) ([]ToolInfo, error) { return nil, nil }
func (m *mockClientFailCall) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return nil, errors.New("call failed")
}
func (m *mockClientFailCall) Close() error { return nil }
