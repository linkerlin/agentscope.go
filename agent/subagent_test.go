package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

type mockAgent struct {
	name    string
	resp    *message.Msg
	err     error
	lastCtx context.Context
}

func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	m.lastCtx = ctx
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func TestSubagentDepth(t *testing.T) {
	ctx := context.Background()
	if d := SubagentDepth(ctx); d != 0 {
		t.Fatalf("expected depth 0, got %d", d)
	}

	ctx = WithSubagentDepth(ctx, 5)
	if d := SubagentDepth(ctx); d != 5 {
		t.Fatalf("expected depth 5, got %d", d)
	}
}

func TestSubagentNewSubagentTool(t *testing.T) {
	mock := &mockAgent{name: "inner"}
	st := NewSubagentTool(mock, "test-tool", "a test tool", 0)

	if st.Name() != "test-tool" {
		t.Errorf("expected name 'test-tool', got %s", st.Name())
	}
	if st.Description() != "a test tool" {
		t.Errorf("expected description 'a test tool', got %s", st.Description())
	}
	if st.maxDepth != 3 {
		t.Errorf("expected default maxDepth 3, got %d", st.maxDepth)
	}

	spec := st.Spec()
	if spec.Name != "test-tool" {
		t.Errorf("expected spec.Name 'test-tool', got %s", spec.Name)
	}
	if spec.Description != "a test tool" {
		t.Errorf("expected spec.Description 'a test tool', got %s", spec.Description)
	}
	if spec.Parameters == nil {
		t.Fatal("expected non-nil Parameters")
	}
}

func TestSubagentToolExecute_Success(t *testing.T) {
	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("hello from subagent").Build(),
	}
	st := NewSubagentTool(mock, "sub", "desc", 3)

	ctx := WithSubagentDepth(context.Background(), 1)
	resp, err := st.Execute(ctx, map[string]any{"query": "do work"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.GetTextContent() != "hello from subagent" {
		t.Errorf("expected response text 'hello from subagent', got %s", resp.GetTextContent())
	}
	if mock.lastCtx == nil {
		t.Fatal("expected mock.lastCtx to be set")
	}
	if d := SubagentDepth(mock.lastCtx); d != 2 {
		t.Errorf("expected depth incremented to 2 in inner agent ctx, got %d", d)
	}
}

func TestSubagentToolExecute_MaxDepthExceeded(t *testing.T) {
	mock := &mockAgent{}
	st := NewSubagentTool(mock, "sub", "desc", 2)

	ctx := WithSubagentDepth(context.Background(), 2)
	_, err := st.Execute(ctx, map[string]any{"query": "do work"})
	if err == nil {
		t.Fatal("expected error for max depth exceeded")
	}
	if !strings.Contains(err.Error(), "max depth") {
		t.Errorf("expected error containing 'max depth', got %v", err)
	}
}

func TestSubagentToolExecute_MissingQuery(t *testing.T) {
	mock := &mockAgent{}
	st := NewSubagentTool(mock, "sub", "desc", 3)

	// missing key
	_, err := st.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
	if !strings.Contains(err.Error(), "missing query") {
		t.Errorf("expected error containing 'missing query', got %v", err)
	}

	// empty string value
	_, err = st.Execute(context.Background(), map[string]any{"query": ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "missing query") {
		t.Errorf("expected error containing 'missing query', got %v", err)
	}
}

var _ tool.Tool = (*SubagentTool)(nil)
