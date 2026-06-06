package react

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/tool"
)

func TestReActAgent_ReplyStream_EmitsEvents(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	mem := memory.NewInMemoryMemory()
	agent, err := Builder().
		Name("test").
		Model(m).
		Memory(mem).
		MaxIterations(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()

	evCh, err := agent.ReplyStream(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}

	var events []event.AgentEvent
	for ev := range evCh {
		events = append(events, ev)
	}

	// Must have at least ReplyStart and ReplyEnd
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[0].EventType() != event.TypeReplyStart {
		t.Fatalf("first event should be reply_start, got %s", events[0].EventType())
	}
	if events[len(events)-1].EventType() != event.TypeReplyEnd {
		t.Fatalf("last event should be reply_end, got %s", events[len(events)-1].EventType())
	}

	// Should contain a text block delta with "ok"
	foundText := false
	for _, ev := range events {
		if d, ok := ev.(*event.TextBlockDeltaEvent); ok && d.Delta == "ok" {
			foundText = true
		}
	}
	if !foundText {
		t.Fatalf("expected text_block_delta with 'ok', got events: %v", events)
	}
}

func TestReActAgent_SaveLoadState(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	mem := memory.NewInMemoryMemory()
	agent, err := Builder().
		Name("test").
		Model(m).
		Memory(mem).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Before a reply, SaveState should fail
	_, err = agent.SaveState()
	if err == nil {
		t.Fatal("expected error when no active runtime state")
	}

	// Start a reply (but don't consume all events) to initialise runtime state
	ctx, cancel := context.WithCancel(context.Background())
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	evCh, _ := agent.ReplyStream(ctx, msg)

	// Give the goroutine time to start
	time.Sleep(50 * time.Millisecond)

	st, err := agent.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	if st.AgentName != "test" {
		t.Fatalf("agent name mismatch: %s", st.AgentName)
	}
	if st.ReplyID == "" {
		t.Fatal("reply_id should be set")
	}

	// Cancel context to end the stream
	cancel()
	for range evCh {
	}
}

func TestReActAgent_InjectEvent_Unsupported(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	mem := memory.NewInMemoryMemory()
	agent, err := Builder().
		Name("test").
		Model(m).
		Memory(mem).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	err = agent.InjectEvent(ctx, event.NewReplyStart("r1", "test"))
	if err == nil {
		t.Fatal("expected error for unsupported event type")
	}
}

func TestReActAgent_ReplyStream_EventBus(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	mem := memory.NewInMemoryMemory()
	bus := event.NewBus(64)

	agent, err := Builder().
		Name("test").
		Model(m).
		Memory(mem).
		MaxIterations(3).
		WithEventBus(bus).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe to the event bus
	subID, subCh := bus.Subscribe()
	defer bus.Unsubscribe(subID)

	ctx := context.Background()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()

	evCh, err := agent.ReplyStream(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Consume the returned channel
	var returnedEvents []event.AgentEvent
	for ev := range evCh {
		returnedEvents = append(returnedEvents, ev)
	}

	// Also collect events from the bus
	var busEvents []event.AgentEvent
	collectDone := time.After(500 * time.Millisecond)
	collect:
	for {
		select {
		case ev, ok := <-subCh:
			if !ok {
				break collect
			}
			busEvents = append(busEvents, ev)
		case <-collectDone:
			break collect
		}
	}

	if len(returnedEvents) < 2 {
		t.Fatalf("expected at least 2 returned events, got %d", len(returnedEvents))
	}
	if len(busEvents) < 2 {
		t.Fatalf("expected at least 2 bus events, got %d", len(busEvents))
	}

	// Both channels should contain the same events
	if len(returnedEvents) != len(busEvents) {
		t.Fatalf("returned %d events but bus got %d", len(returnedEvents), len(busEvents))
	}
	for i := range returnedEvents {
		if returnedEvents[i].EventType() != busEvents[i].EventType() {
			t.Fatalf("event type mismatch at %d: %s vs %s", i, returnedEvents[i].EventType(), busEvents[i].EventType())
		}
	}
}


// mockTool is a simple tool for testing HITL flows.
type mockTool struct {
	name   string
	result string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool for testing" }
func (t *mockTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.name,
		Description: t.Description(),
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}
func (t *mockTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return tool.NewTextResponse(t.result), nil
}

// hitlMockModel returns a message with a ToolUseBlock on the first call,
// then a plain text response on subsequent calls.
type hitlMockModel struct {
	toolName  string
	callCount int
}

func (m *hitlMockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.callCount++
	if m.callCount == 1 {
		return message.NewMsg().Role(message.RoleAssistant).
			Content(message.NewToolUseBlock("tc1", m.toolName, map[string]any{})).
			Build(), nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("done").Build(), nil
}

func (m *hitlMockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "ok"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *hitlMockModel) ModelName() string { return "mock-tool-model" }

func TestReActAgent_HITL_PermissionAsk(t *testing.T) {
	mockT := &mockTool{name: "mock_tool", result: "tool-result"}
	m := &hitlMockModel{toolName: "mock_tool"}
	pe := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Name: "ask-mock", Target: "tool_name", Pattern: "mock_tool", Decision: permission.DecisionAsk},
	})

	agent, err := Builder().
		Name("test").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(mockT).
		PermissionEngine(pe).
		MaxIterations(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("call tool").Build()
	evCh, err := agent.ReplyStream(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}

	var foundConfirm bool
	for ev := range evCh {
		if confirm, ok := ev.(*event.RequireUserConfirmEvent); ok {
			foundConfirm = true
			// Inject allow decision for the tool call
			decisions := []event.ConfirmDecision{
				{ToolCallID: confirm.ToolCalls[0].ID, Decision: "allow"},
			}
			_ = agent.InjectEvent(ctx, event.NewUserConfirmResult(confirm.ReplyID(), confirm.ConfirmID, decisions))
		}
	}

	if !foundConfirm {
		t.Fatal("expected RequireUserConfirmEvent")
	}
}

func TestReActAgent_HITL_DenyAll(t *testing.T) {
	mockT := &mockTool{name: "mock_tool", result: "tool-result"}
	m := &hitlMockModel{toolName: "mock_tool"}
	pe := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Name: "ask-mock", Target: "tool_name", Pattern: "mock_tool", Decision: permission.DecisionAsk},
	})

	agent, err := Builder().
		Name("test").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(mockT).
		PermissionEngine(pe).
		MaxIterations(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("call tool").Build()
	evCh, err := agent.ReplyStream(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}

	var foundConfirm bool
	for ev := range evCh {
		if confirm, ok := ev.(*event.RequireUserConfirmEvent); ok {
			foundConfirm = true
			// Inject deny decision for the tool call
			decisions := []event.ConfirmDecision{
				{ToolCallID: confirm.ToolCalls[0].ID, Decision: "deny"},
			}
			_ = agent.InjectEvent(ctx, event.NewUserConfirmResult(confirm.ReplyID(), confirm.ConfirmID, decisions))
		}
	}

	if !foundConfirm {
		t.Fatal("expected RequireUserConfirmEvent")
	}
}
