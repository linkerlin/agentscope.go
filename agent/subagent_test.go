package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/state"
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
	if !strings.Contains(err.Error(), "missing message") {
		t.Errorf("expected error containing 'missing message', got %v", err)
	}

	// empty string value
	_, err = st.Execute(context.Background(), map[string]any{"query": ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "missing message") {
		t.Errorf("expected error containing 'missing message', got %v", err)
	}
}

func TestSubagentToolExecute_SessionMode(t *testing.T) {
	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("session reply").Build(),
	}
	st := NewSubagentTool(mock, "sub", "desc", 3)

	resp, err := st.Execute(context.Background(), map[string]any{"session_id": "s1", "message": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "session_id: s1") {
		t.Fatalf("expected session_id in response, got: %s", resp.GetTextContent())
	}
	if !strings.Contains(resp.GetTextContent(), "session reply") {
		t.Fatalf("expected reply in response, got: %s", resp.GetTextContent())
	}

	// history accumulated
	st.mu.Lock()
	if len(st.sessions["s1"]) != 2 {
		t.Fatalf("expected 2 messages in session, got %d", len(st.sessions["s1"]))
	}
	st.mu.Unlock()
}

func TestSubagentToolExecute_SessionAutoID(t *testing.T) {
	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("auto").Build(),
	}
	st := NewSubagentTool(mock, "sub", "desc", 3)

	resp, err := st.Execute(context.Background(), map[string]any{"message": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.GetTextContent(), "session_id: sess_") {
		t.Fatalf("expected auto session id, got: %s", resp.GetTextContent())
	}
}

func TestSubagentToolExecute_WithProvider(t *testing.T) {
	callCount := 0
	st := NewSubagentToolWithProvider(func() Agent {
		callCount++
		return &mockAgent{
			resp: message.NewMsg().Role(message.RoleAssistant).TextContent("provided").Build(),
		}
	}, "sub", "desc", 3)

	resp, err := st.Execute(context.Background(), map[string]any{"message": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("expected provider called once, got %d", callCount)
	}
	if !strings.Contains(resp.GetTextContent(), "provided") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestSubagentSessionTool_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("reply1").Build(),
	}
	sst := NewSubagentSessionTool(mock, "sub", "desc", 3, store)

	// first call
	resp, err := sst.Execute(context.Background(), map[string]any{"session_id": "s1", "message": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "reply1") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}

	// verify saved
	exists := store.Exists("s1")
	if !exists {
		t.Fatal("expected session s1 to be saved")
	}

	// create new tool instance and load
	mock2 := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("reply2").Build(),
	}
	sst2 := NewSubagentSessionTool(mock2, "sub", "desc", 3, store)
	resp2, err := sst2.Execute(context.Background(), map[string]any{"session_id": "s1", "message": "again"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp2.GetTextContent(), "reply2") {
		t.Fatalf("unexpected response: %s", resp2.GetTextContent())
	}

	// verify history loaded (2 previous + 1 new user + 1 new assistant = 4)
	sst2.mu.Lock()
	if len(sst2.sessions["s1"]) != 4 {
		t.Fatalf("expected 4 messages in loaded session, got %d", len(sst2.sessions["s1"]))
	}
	sst2.mu.Unlock()
}

func TestSubagentSessionTool_LoadIfExists_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
	}
	sst := NewSubagentSessionTool(mock, "sub", "desc", 3, store)

	ok, err := sst.LoadIfExists(store, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected LoadIfExists to return false for missing key")
	}
}

func TestSubagentSessionTool_NilStore(t *testing.T) {
	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
	}
	sst := NewSubagentSessionTool(mock, "sub", "desc", 3, nil)

	resp, err := sst.Execute(context.Background(), map[string]any{"session_id": "s1", "message": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "ok") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}

	// SaveTo/LoadFrom with nil store should not panic
	if err := sst.SaveTo(nil, "k"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := sst.LoadFrom(nil, "k"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubagentSessionTool_SaveTo_LoadFrom_FileStore(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(),
	}
	sst := NewSubagentSessionTool(mock, "sub", "desc", 3, store)

	_, err = sst.Execute(context.Background(), map[string]any{"session_id": "abc", "message": "msg"})
	if err != nil {
		t.Fatal(err)
	}

	if err := sst.SaveTo(store, "abc"); err != nil {
		t.Fatal(err)
	}

	// verify file exists
	path := dir + string(os.PathSeparator) + "abc.json"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected JSON file to exist")
	}

	// load into new instance
	sst2 := NewSubagentSessionTool(mock, "sub", "desc", 3, store)
	if err := sst2.LoadFrom(store, "abc"); err != nil {
		t.Fatal(err)
	}
	sst2.mu.Lock()
	if len(sst2.sessions["abc"]) != 2 {
		t.Fatalf("expected 2 messages after load, got %d", len(sst2.sessions["abc"]))
	}
	sst2.mu.Unlock()
}

var _ tool.Tool = (*SubagentTool)(nil)
var _ tool.Tool = (*SubagentSessionTool)(nil)
