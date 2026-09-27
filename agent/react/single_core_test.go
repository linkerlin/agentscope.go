package react

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// Golden-parity tests for the 16.1 single-core loop: the synchronous Call path
// is a collector over the same unified loop ReplyStream drives, so both must
// agree on the final response, hook termination and steer semantics.

// TestCall_MatchesReplyStreamCollection runs the same scripted turn through
// Call and through manual ReplyStream collection on two identical agents and
// asserts both land on the same final assistant message.
func TestCall_MatchesReplyStreamCollection(t *testing.T) {
	echoTool := tool.NewFunctionTool("echo", "echo", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("done"), nil
	})
	build := func() *ReActAgent {
		toolCall := message.NewMsg().Role(message.RoleAssistant).Content(
			message.NewToolUseBlock("call_1", "echo", map[string]any{"x": 1}),
		).Build()
		final := message.NewMsg().Role(message.RoleAssistant).TextContent("final answer").Build()
		m := &mockToolModel{name: "m", responses: []*message.Msg{toolCall, final}}
		a, err := Builder().Name("Test").Model(m).Tools(echoTool).Build()
		if err != nil {
			t.Fatal(err)
		}
		return a
	}

	respCall, err := build().Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}

	a2 := build()
	ch, err := a2.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	a2.runtimeMu.Lock()
	var respStream *message.Msg
	for i := len(a2.runtimeState.Messages) - 1; i >= 0; i-- {
		if a2.runtimeState.Messages[i].Role == message.RoleAssistant {
			respStream = a2.runtimeState.Messages[i]
			break
		}
	}
	a2.runtimeMu.Unlock()

	if respCall.GetTextContent() != "final answer" || respStream.GetTextContent() != "final answer" {
		t.Fatalf("Call/stream collectors disagree: %q vs %q", respCall.GetTextContent(), respStream.GetTextContent())
	}
}

// stopAfterToolHook terminates the turn from the after-tool hook point.
type stopAfterToolHook struct{}

func (h *stopAfterToolHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if hCtx.Point == hook.HookAfterTool {
		return &hook.HookResult{
			StopAgent: true,
			Override:  message.NewMsg().Role(message.RoleAssistant).TextContent("stopped by hook").Build(),
		}, nil
	}
	return nil, nil
}

// TestCall_HookStopAgentTerminatesTurn verifies the hook StopAgent semantics
// on the unified loop: the override becomes the turn's final response and is
// never fed back into the loop as a tool result (V1 Call parity).
func TestCall_HookStopAgentTerminatesTurn(t *testing.T) {
	echoTool := tool.NewFunctionTool("echo", "echo", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("done"), nil
	})
	toolCall := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{}),
	).Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCall}}
	a, err := Builder().Name("Test").Model(m).Tools(echoTool).Hooks(&stopAfterToolHook{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "stopped by hook" {
		t.Fatalf("expected hook override as final response, got %q", resp.GetTextContent())
	}
}

// TestCall_SteerAcceptedDuringTurn verifies Steer works on the synchronous
// path too: the unified loop registers the active turn before the goroutine
// starts, so a mid-turn steer is accepted instead of rejected with
// "no active turn" (which was the pre-16.1 Call behavior).
func TestCall_SteerAcceptedDuringTurn(t *testing.T) {
	m := &slowMockChatModel{name: "mock", delay: 80 * time.Millisecond}
	a, err := Builder().Name("Test").Model(m).MaxIterations(1).Build()
	if err != nil {
		t.Fatal(err)
	}
	steered := make(chan error, 1)
	go func() {
		time.Sleep(15 * time.Millisecond)
		steered <- a.Steer("mid-turn note")
	}()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if err := <-steered; err != nil {
		t.Fatalf("steer during a synchronous turn was rejected: %v", err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected response: %q", resp.GetTextContent())
	}
}
