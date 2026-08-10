package react

import (
	"context"
	"errors"
	"strconv"
	"strings"
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
	subID, subCh, _ := bus.Subscribe()
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
	msg, err := m.Chat(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	ch := make(chan *model.StreamChunk, 4)
	if text := msg.GetTextContent(); text != "" {
		ch <- &model.StreamChunk{Delta: text}
	}
	content := msg.Content
	if len(content) > 0 {
		ch <- &model.StreamChunk{Done: true, Content: content}
	} else {
		ch <- &model.StreamChunk{Done: true}
	}
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

// oddToolModel emits a tool-use on odd-numbered Chat calls and a plain "done"
// on even ones, so two turns each see exactly one tool call followed by a
// final answer. Used to verify session-scoped ("always") approvals persist
// across turns.
type oddToolModel struct {
	toolName string
	calls    int
}

func (m *oddToolModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.calls++
	if m.calls%2 == 1 {
		return message.NewMsg().Role(message.RoleAssistant).
			Content(message.NewToolUseBlock("tc"+strconv.Itoa(m.calls), m.toolName, map[string]any{})).
			Build(), nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("done").Build(), nil
}

func (m *oddToolModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	msg, err := m.Chat(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	ch := make(chan *model.StreamChunk, 4)
	if text := msg.GetTextContent(); text != "" {
		ch <- &model.StreamChunk{Delta: text}
	}
	if len(msg.Content) > 0 {
		ch <- &model.StreamChunk{Done: true, Content: msg.Content}
	} else {
		ch <- &model.StreamChunk{Done: true}
	}
	close(ch)
	return ch, nil
}

func (m *oddToolModel) ModelName() string { return "odd-tool-model" }

// TestReActAgent_HITL_AlwaysApprovalPersistsAcrossTurns reproduces kopaw's real
// config: ModeDefault (no explicit ask rule), so a non-readonly tool asks via
// the mode default and the eval carries Rule == nil. After the user approves
// with scope "always", the SAME tool must NOT ask again on the next turn.
// Regression: previously the approval was silently dropped because the
// registration code required ev.Rule != nil.
func TestReActAgent_HITL_AlwaysApprovalPersistsAcrossTurns(t *testing.T) {
	mockT := &mockTool{name: "mock_tool", result: "tool-result"}
	m := &oddToolModel{toolName: "mock_tool"}
	// ModeDefault + NO ask rule: the tool asks purely via the mode default,
	// exactly like kopaw's AUTO approval level. ev.Rule is nil for such asks.
	pe := permission.NewEngine(permission.ModeDefault, nil)
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

	// Turn 1: expect exactly one confirm; approve with scope "always".
	ctx := context.Background()
	ch1, err := agent.ReplyStream(ctx, message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	if err != nil {
		t.Fatal(err)
	}
	var turn1Confirms int
	for ev := range ch1 {
		if confirm, ok := ev.(*event.RequireUserConfirmEvent); ok {
			turn1Confirms++
			_ = agent.InjectEvent(ctx, event.NewUserConfirmResult(confirm.ReplyID(), confirm.ConfirmID, []event.ConfirmDecision{
				{ToolCallID: confirm.ToolCalls[0].ID, Decision: "always_allow", Scope: "always"},
			}))
		}
	}
	if turn1Confirms != 1 {
		t.Fatalf("turn 1: expected exactly 1 confirm, got %d", turn1Confirms)
	}

	// Turn 2: the "always" approval must persist — no confirm expected. Use a
	// timeout so a regression (approval not registered → asks again → blocks
	// waiting for a decision) fails fast instead of hanging the test runner.
	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch2, err := agent.ReplyStream(ctx2, message.NewMsg().Role(message.RoleUser).TextContent("again").Build())
	if err != nil {
		t.Fatal(err)
	}
	var turn2Confirms int
	for ev := range ch2 {
		if confirm, ok := ev.(*event.RequireUserConfirmEvent); ok {
			turn2Confirms++
			// Drain the confirm so the stream can close instead of blocking.
			_ = agent.InjectEvent(ctx2, event.NewUserConfirmResult(confirm.ReplyID(), confirm.ConfirmID, []event.ConfirmDecision{
				{ToolCallID: confirm.ToolCalls[0].ID, Decision: "allow"},
			}))
		}
	}
	if turn2Confirms != 0 {
		t.Fatalf("turn 2: expected 0 confirms (always should persist), got %d", turn2Confirms)
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
func TestReActAgent_Reply_ReturnsFinalMsg(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	agent, err := Builder().
		Name("test").
		Model(m).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	resp, err := agent.Reply(context.Background(), msg)
	if err != nil {
		t.Fatalf("reply failed: %v", err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("expected 'ok', got %q", resp.GetTextContent())
	}
	if resp.Role != message.RoleAssistant {
		t.Fatalf("expected assistant role, got %s", resp.Role)
	}
}

func TestReActAgent_Reply_ExceedMaxIters(t *testing.T) {
	m := &mockToolModel{
		responses: []*message.Msg{
			message.NewMsg().Role(message.RoleAssistant).Content(
				message.NewToolUseBlock("tc1", "dummy_tool", map[string]any{}),
			).Build(),
		},
	}
	agent, err := Builder().
		Name("test").
		Model(m).
		MaxIterations(1).
		Tools(&dummyTool{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent("trigger tool").Build()
	_, err = agent.Reply(context.Background(), msg)
	if err == nil || err.Error() != "react agent: max iterations reached without final answer" {
		t.Fatalf("expected max iters error, got %v", err)
	}
}

func TestReActAgent_Reply_EmitsModelCallEvents(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	agent, err := Builder().
		Name("test").
		Model(m).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}

	var gotModelCallStart, gotModelCallEnd bool
	for ev := range ch {
		if ev.EventType() == "model_call_start" {
			gotModelCallStart = true
		}
		if ev.EventType() == "model_call_end" {
			gotModelCallEnd = true
		}
	}
	if !gotModelCallStart {
		t.Fatal("expected model_call_start event")
	}
	if !gotModelCallEnd {
		t.Fatal("expected model_call_end event")
	}
}

func TestReActAgent_Reply_ExceedMaxItersEvent(t *testing.T) {
	m := &mockToolModel{
		responses: []*message.Msg{
			message.NewMsg().Role(message.RoleAssistant).Content(
				message.NewToolUseBlock("tc1", "dummy_tool", map[string]any{}),
			).Build(),
		},
	}
	agent, err := Builder().
		Name("test").
		Model(m).
		MaxIterations(1).
		Tools(&dummyTool{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("trigger").Build())
	if err != nil {
		t.Fatal(err)
	}

	var gotExceed bool
	for ev := range ch {
		if ev.EventType() == "exceed_max_iters" {
			gotExceed = true
		}
	}
	if !gotExceed {
		t.Fatal("expected exceed_max_iters event")
	}
}

// dummyTool is a no-op tool for testing max iters.
type dummyTool struct{}

func (d *dummyTool) Name() string        { return "dummy_tool" }
func (d *dummyTool) Description() string { return "dummy" }
func (d *dummyTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        d.Name(),
		Description: d.Description(),
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
}
func (d *dummyTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return tool.NewTextResponse("done"), nil
}

var _ tool.Tool = (*dummyTool)(nil)

// errStreamChatModel 发一个流中途错误 chunk 的 mock 模型。
type errStreamChatModel struct {
	err error
}

func (m *errStreamChatModel) ModelName() string { return "err" }
func (m *errStreamChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return nil, m.err
}
func (m *errStreamChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "partial"}
	ch <- &model.StreamChunk{Done: true, Error: m.err}
	close(ch)
	return ch, nil
}

// TestReActAgent_ReplyStream_MidStreamError 验证 V2 流中途错误以 ErrorEvent 暴露,
// 而不是静默产出空回复。
func TestReActAgent_ReplyStream_MidStreamError(t *testing.T) {
	streamErr := errors.New("mid stream boom")
	m := &errStreamChatModel{err: streamErr}
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

	evCh, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	var gotErr string
	for ev := range evCh {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			gotErr = e.Err
		}
	}
	if gotErr == "" {
		t.Fatal("expected error event in stream")
	}
	if !strings.Contains(gotErr, "mid stream boom") {
		t.Fatalf("expected error message to propagate, got %q", gotErr)
	}
}
