package a2atool

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/a2a"
)

// --- Mock A2A Client ---

type mockA2AClient struct {
	sendResp  *a2a.Message
	sendErr   error
	streamCh  chan *a2a.Message
	streamErr error
	sentMsg   *a2a.Message
}

func (m *mockA2AClient) Send(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
	m.sentMsg = msg
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	return m.sendResp, nil
}

func (m *mockA2AClient) SendSubscribe(ctx context.Context, msg *a2a.Message) (<-chan *a2a.Message, error) {
	m.sentMsg = msg
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return m.streamCh, nil
}

func (m *mockA2AClient) Close() error { return nil }

// --- Tests ---

func TestA2ATool_Name(t *testing.T) {
	tool := NewA2ATool("remote-agent", "A remote helper", &mockA2AClient{})
	if tool.Name() != "remote-agent" {
		t.Fatalf("expected name 'remote-agent', got '%s'", tool.Name())
	}
}

func TestA2ATool_Spec(t *testing.T) {
	tool := NewA2ATool("ra", "desc", &mockA2AClient{})
	spec := tool.Spec()
	if spec.Name != "ra" {
		t.Fatalf("expected spec name 'ra'")
	}
	props, ok := spec.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["task"]; !ok {
		t.Error("expected 'task' property")
	}
	if _, ok := props["session_id"]; !ok {
		t.Error("expected 'session_id' property")
	}
}

func TestA2ATool_ExecuteSync(t *testing.T) {
	client := &mockA2AClient{
		sendResp: &a2a.Message{Role: "agent", Content: "Task completed successfully"},
	}
	tool := NewA2ATool("remote", "A remote agent", client)

	resp, err := tool.Execute(context.Background(), map[string]any{
		"task": "Do something",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "Task completed successfully" {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	if client.sentMsg.Content != "Do something" {
		t.Errorf("expected task sent to client, got '%s'", client.sentMsg.Content)
	}
}

func TestA2ATool_ExecuteSync_EmptyResponse(t *testing.T) {
	client := &mockA2AClient{
		sendResp: &a2a.Message{Role: "agent", Content: ""},
	}
	tool := NewA2ATool("remote", "desc", client)

	resp, err := tool.Execute(context.Background(), map[string]any{
		"task": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "empty response") {
		t.Fatalf("expected error about empty response, got: %s", resp.GetTextContent())
	}
}

func TestA2ATool_ExecuteSync_Error(t *testing.T) {
	client := &mockA2AClient{
		sendErr: errors.New("network timeout"),
	}
	tool := NewA2ATool("remote", "desc", client)

	resp, err := tool.Execute(context.Background(), map[string]any{
		"task": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "network timeout") {
		t.Fatalf("expected error in response, got: %s", resp.GetTextContent())
	}
}

func TestA2ATool_Execute_NoTask(t *testing.T) {
	tool := NewA2ATool("remote", "desc", &mockA2AClient{})
	resp, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "task is required") {
		t.Fatalf("expected 'task is required' error, got: %s", resp.GetTextContent())
	}
}

func TestA2ATool_ExecuteStreaming(t *testing.T) {
	ch := make(chan *a2a.Message, 3)
	ch <- &a2a.Message{Content: "Working..."}
	ch <- &a2a.Message{Content: "Almost done..."}
	ch <- &a2a.Message{Content: "Done!"}
	close(ch)

	client := &mockA2AClient{streamCh: ch}
	tool := NewA2ATool("remote", "desc", client).WithStreaming(true)

	var progressMsgs []string
	tool.WithProgressFn(func(delta string) {
		progressMsgs = append(progressMsgs, delta)
	})

	resp, err := tool.Execute(context.Background(), map[string]any{
		"task": "Stream test",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "Working...") {
		t.Error("expected 'Working...' in response")
	}
	if !strings.Contains(text, "Done!") {
		t.Error("expected 'Done!' in response")
	}
	if len(progressMsgs) != 3 {
		t.Fatalf("expected 3 progress calls, got %d", len(progressMsgs))
	}
}

func TestA2ATool_ExecuteStreaming_EmptyStream(t *testing.T) {
	ch := make(chan *a2a.Message)
	close(ch)

	client := &mockA2AClient{streamCh: ch}
	tool := NewA2ATool("remote", "desc", client).WithStreaming(true)

	resp, err := tool.Execute(context.Background(), map[string]any{
		"task": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "empty stream") {
		t.Fatalf("expected empty stream error, got: %s", resp.GetTextContent())
	}
}

func TestA2ATool_WithSessionID(t *testing.T) {
	client := &mockA2AClient{
		sendResp: &a2a.Message{Role: "agent", Content: "ok"},
	}
	tool := NewA2ATool("remote", "desc", client)

	_, _ = tool.Execute(context.Background(), map[string]any{
		"task":       "test",
		"session_id": "sess-123",
	})
	if client.sentMsg.Meta == nil || client.sentMsg.Meta["session_id"] != "sess-123" {
		t.Error("expected session_id in meta")
	}
}

func TestA2ATool_Description_Streaming(t *testing.T) {
	tool := NewA2ATool("remote", "desc", &mockA2AClient{})
	syncDesc := tool.Description()
	if !strings.Contains(syncDesc, "synchronous") {
		t.Errorf("expected 'synchronous' in desc, got: %s", syncDesc)
	}

	tool.WithStreaming(true)
	streamDesc := tool.Description()
	if !strings.Contains(streamDesc, "streaming") {
		t.Errorf("expected 'streaming' in desc, got: %s", streamDesc)
	}
}

// --- Registry Tests ---

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry(func(url string) a2a.Client {
		return &mockA2AClient{sendResp: &a2a.Message{Content: "ok"}}
	})
	r.Register("coder", "Writes code", "http://localhost:8080")

	ra, ok := r.Get("coder")
	if !ok {
		t.Fatal("expected to find 'coder'")
	}
	if ra.URL != "http://localhost:8080" {
		t.Errorf("unexpected URL: %s", ra.URL)
	}

	_, ok = r.Get("unknown")
	if ok {
		t.Error("expected 'unknown' to not be found")
	}
}

func TestRegistry_CreateTool(t *testing.T) {
	var lastURL string
	r := NewRegistry(func(url string) a2a.Client {
		lastURL = url
		return &mockA2AClient{sendResp: &a2a.Message{Content: "result"}}
	})
	r.Register("analyst", "Analyzes data", "http://analyst:8080")

	tool, err := r.CreateTool("analyst")
	if err != nil {
		t.Fatal(err)
	}
	if tool.Name() != "analyst" {
		t.Errorf("expected name 'analyst', got '%s'", tool.Name())
	}
	if lastURL != "http://analyst:8080" {
		t.Errorf("expected factory called with URL, got '%s'", lastURL)
	}

	_, err = r.CreateTool("unknown")
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestRegistry_ClientCaching(t *testing.T) {
	callCount := 0
	r := NewRegistry(func(url string) a2a.Client {
		callCount++
		return &mockA2AClient{sendResp: &a2a.Message{Content: "ok"}}
	})
	r.Register("a1", "Agent 1", "http://a1:8080")
	r.Register("a2", "Agent 2", "http://a2:8080")

	_, _ = r.CreateTool("a1")
	_, _ = r.CreateTool("a1")
	_, _ = r.CreateTool("a2")

	if callCount != 2 {
		t.Fatalf("expected 2 factory calls (cached), got %d", callCount)
	}
}

func TestRegistry_AllTools(t *testing.T) {
	r := NewRegistry(func(url string) a2a.Client {
		return &mockA2AClient{sendResp: &a2a.Message{Content: "ok"}}
	})
	r.Register("a1", "Agent 1", "http://a1:8080")
	r.Register("a2", "Agent 2", "http://a2:8080")
	r.Register("a3", "Agent 3", "http://a3:8080")

	tools := r.AllTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}

func TestRegistry_RegisterFromAgentCards(t *testing.T) {
	r := NewRegistry(nil)
	cards := []a2a.AgentCard{
		{Name: "agent1", Description: "First agent", URL: "http://a1:8080"},
		{Name: "agent2", Description: "Second agent", URL: "http://a2:8080"},
	}
	r.RegisterFromAgentCards(cards)

	if _, ok := r.Get("agent1"); !ok {
		t.Error("expected 'agent1' to be registered")
	}
	if _, ok := r.Get("agent2"); !ok {
		t.Error("expected 'agent2' to be registered")
	}
}

func TestRegistry_Close(t *testing.T) {
	r := NewRegistry(func(url string) a2a.Client {
		return &mockA2AClient{sendResp: &a2a.Message{Content: "ok"}}
	})
	r.Register("a1", "desc", "http://a1:8080")
	_, _ = r.CreateTool("a1")

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestRegistry_Ping(t *testing.T) {
	r := NewRegistry(func(url string) a2a.Client {
		return &mockA2AClient{sendResp: &a2a.Message{Content: "pong"}}
	})
	r.Register("a1", "desc", "http://a1:8080")

	if err := r.Ping(context.Background(), "a1"); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if err := r.Ping(context.Background(), "unknown"); err == nil {
		t.Error("expected error for unknown agent")
	}
}
