package react

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/tool"
)

// resumeMockModel returns a tool-call on first Chat, then a final answer.
type resumeMockModel struct {
	calls int
}

func (m *resumeMockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.calls++
	if m.calls == 1 {
		return message.NewMsg().Role(message.RoleAssistant).Content(
			message.NewToolUseBlock("call_1", "echo", map[string]any{"x": 1}),
		).Build(), nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("resumed_ok").Build(), nil
}

func (m *resumeMockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "stream"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *resumeMockModel) ModelName() string { return "resume-mock" }

func TestReplyStream_ReconnectResume(t *testing.T) {
	echoTool := tool.NewFunctionTool("echo", "echo", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("echo_result"), nil
	})

	// Permission engine that ASKs for every tool call
	permEngine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Target: "echo", Pattern: "*", Decision: permission.DecisionAsk},
	})

	m := &resumeMockModel{}
	a, err := Builder().
		Name("resume-test").
		Model(m).
		Tools(echoTool).
		PermissionEngine(permEngine).
		MaxIterations(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// ---- First connection: start stream and let it suspend ----
	ctx1, cancel1 := context.WithCancel(context.Background())
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	ch1, err := a.ReplyStream(ctx1, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Consume events until we see RequireUserConfirmEvent
	var sawSuspend bool
	for ev := range ch1 {
		if _, ok := ev.(*event.RequireUserConfirmEvent); ok {
			sawSuspend = true
			break
		}
	}
	if !sawSuspend {
		t.Fatal("expected RequireUserConfirmEvent")
	}

	// Save state while suspended
	st, err := a.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	if st.SuspendedAt == nil {
		t.Fatal("expected SuspendedAt to be set")
	}
	if st.WaitConfirmID == "" {
		t.Fatal("expected WaitConfirmID to be set")
	}

	// Cancel first context (simulates disconnect)
	cancel1()
	for range ch1 {
	}

	// ---- Reconnect: LoadState and restart stream ----
	if err := a.LoadState(st); err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	ctx2 := context.Background()
	ch2, err := a.ReplyStream(ctx2, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Inject confirmation in background so the resumed loop can proceed
	go func() {
		// Small delay to ensure the resumed loop has entered waitForExternalEvent
		time.Sleep(50 * time.Millisecond)
		_ = a.InjectEvent(ctx2, event.NewUserConfirmResult(st.ReplyID, st.WaitConfirmID, []event.ConfirmDecision{
			{ToolCallID: "call_1", Decision: "allow"},
		}))
	}()

	// Consume resumed events
	var sawToolResult, sawReplyEnd bool
	var lastError string
	for ev := range ch2 {
		switch e := ev.(type) {
		case *event.ToolResultStartEvent:
			sawToolResult = true
		case *event.ReplyEndEvent:
			sawReplyEnd = true
		case *event.ErrorEvent:
			lastError = e.Err
		}
	}
	if lastError != "" {
		t.Logf("error event received: %s", lastError)
	}

	if !sawToolResult {
		t.Fatalf("expected ToolResultStartEvent after resume, last error: %s", lastError)
	}
	if !sawReplyEnd {
		t.Fatalf("expected ReplyEndEvent after resume, last error: %s", lastError)
	}
}
