package e2e

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
)

// mustPersistSession registers a session record owned by the env user so
// tests referencing a fixed session ID pass the 22.2 known-session check.
func mustPersistSession(t *testing.T, e *env, sessionID string) {
	t.Helper()
	if err := e.storage.SaveSession(context.Background(), &service.Session{
		ID: sessionID, UserID: e.userID,
	}); err != nil {
		t.Fatal(err)
	}
}

// scriptedModel returns a tool call on the first Chat, then a final answer.
type scriptedModel struct {
	n atomic.Int32
}

func (m *scriptedModel) ModelName() string { return "scripted-e2e" }

func (m *scriptedModel) Chat(_ context.Context, _ []*message.Msg, _ ...model.ChatOption) (*message.Msg, error) {
	n := m.n.Add(1)
	if n == 1 {
		return message.NewMsg().Role(message.RoleAssistant).Content(
			message.NewToolUseBlock("call_1", "add", map[string]any{"a": 1.0, "b": 2.0}),
		).Build(), nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("sum is 3").Build(), nil
}

func (m *scriptedModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	out, err := m.Chat(ctx, msgs, opts...)
	if err != nil {
		return nil, err
	}
	ch := make(chan *model.StreamChunk, 2)
	// V2 ReAct reads tool calls from the Done chunk's Content (not Chat()).
	ch <- &model.StreamChunk{Content: out.Content, Delta: out.GetTextContent(), Done: true}
	close(ch)
	return ch, nil
}

func TestE2E_ReActToolLoopOverHTTP(t *testing.T) {
	add := tool.NewFunctionTool("add", "add two numbers", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}, func(_ context.Context, input map[string]any) (*tool.Response, error) {
		a, _ := input["a"].(float64)
		b, _ := input["b"].(float64)
		return tool.NewTextResponse(a + b), nil
	})
	ag, err := react.Builder().
		Name("calc").
		SysPrompt("Use the add tool.").
		Model(&scriptedModel{}).
		Tools(add).
		MaxIterations(4).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	e := newEnv(t, func(cfg *gateway.AppConfig, _ *streamAgent) {
		cfg.Agent = ag
	})

	code, body := e.mustDo(http.MethodPost, "/v2/chat/stream", map[string]string{"text": "1+2"})
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "sum is 3") && !contain(body, "3") {
		t.Fatalf("react loop did not produce final answer: %s", body)
	}
	if !contain(body, "add") && !contain(body, "tool") {
		t.Fatalf("react loop missing tool events: %s", body)
	}
}

func TestE2E_InterruptActiveTurn(t *testing.T) {
	e := newEnv(t, func(_ *gateway.AppConfig, ag *streamAgent) {
		ag.delay = 1500 * time.Millisecond
	})

	// With storage configured, the referenced session must exist (22.2).
	mustPersistSession(t, e, "slow-sess")

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = e.mustDo(http.MethodPost, "/v2/chat/stream", map[string]string{
			"text": "slow", "session_id": "slow-sess",
		})
	}()

	waitFor(t, time.Second, func() bool {
		code, _ := e.mustDo(http.MethodPost, "/v2/sessions/slow-sess/interrupt", map[string]any{})
		return code == http.StatusOK
	})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not return after interrupt")
	}
}
