package subagent

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// mockAgent is a minimal agent.Agent implementation for testing.
type mockAgent struct {
	name string
	resp string
	err  error
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.resp).Build(), nil
}
func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func TestSubagentTool_BasicExecute(t *testing.T) {
	mock := &mockAgent{name: "math-agent", resp: "42"}
	st := NewSubagentTool("solve_math", "Solves math problems", mock)

	resp, err := st.Execute(context.Background(), map[string]any{"task": "what is 6*7?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "42" {
		t.Fatalf("expected 42, got %q", resp.GetTextContent())
	}
}

func TestSubagentTool_MissingTask(t *testing.T) {
	mock := &mockAgent{name: "math-agent", resp: "42"}
	st := NewSubagentTool("solve_math", "Solves math problems", mock)

	resp, err := st.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error response for missing task")
	}
}

func TestSubagentTool_AgentError(t *testing.T) {
	mock := &mockAgent{name: "failing-agent", err: context.Canceled}
	st := NewSubagentTool("fail", "Always fails", mock)

	resp, err := st.Execute(context.Background(), map[string]any{"task": "do something"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error response")
	}
}

func TestSubagentTool_MaxDepth(t *testing.T) {
	// Create two subagents that recursively call each other.
	agentA := &mockAgent{name: "agent-a", resp: "A"}
	agentB := &mockAgent{name: "agent-b", resp: "B"}

	toolA := NewSubagentTool("delegate_to_b", "Delegates to B", agentB).WithMaxDepth(2)
	_ = NewSubagentTool("delegate_to_a", "Delegates to A", agentA).WithMaxDepth(2)

	// Manually simulate recursive invocation by injecting depth into context.
	// Depth 0 -> depth 1 (OK)
	ctx := context.WithValue(context.Background(), depthKey{}, 0)
	resp, err := toolA.Execute(ctx, map[string]any{"task": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "B" {
		t.Fatalf("expected B, got %q", resp.GetTextContent())
	}

	// Depth 1 -> depth 2 (OK, at limit)
	ctx = context.WithValue(context.Background(), depthKey{}, 1)
	resp, err = toolA.Execute(ctx, map[string]any{"task": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "B" {
		t.Fatalf("expected B, got %q", resp.GetTextContent())
	}

	// Depth 2 -> depth 3 (EXCEEDS limit)
	ctx = context.WithValue(context.Background(), depthKey{}, 2)
	resp, err = toolA.Execute(ctx, map[string]any{"task": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "B" {
		t.Fatal("expected depth limit error, got normal response")
	}
}

func TestSubagentTool_NameAndSpec(t *testing.T) {
	mock := &mockAgent{name: "writer", resp: "ok"}
	st := NewSubagentTool("write_doc", "Writes documentation", mock)

	if st.Name() != "write_doc" {
		t.Fatalf("expected write_doc, got %s", st.Name())
	}
	spec := st.Spec()
	if spec.Name != "write_doc" {
		t.Fatalf("expected spec name write_doc, got %s", spec.Name)
	}
}

func TestSubagentTool_WithMaxDepth(t *testing.T) {
	mock := &mockAgent{name: "a", resp: "x"}
	st := NewSubagentTool("t", "d", mock).WithMaxDepth(5)
	if st.maxDepth != 5 {
		t.Fatalf("expected maxDepth 5, got %d", st.maxDepth)
	}
}

// compile-time check: SubagentTool implements tool.Tool
var _ tool.Tool = (*SubagentTool)(nil)
