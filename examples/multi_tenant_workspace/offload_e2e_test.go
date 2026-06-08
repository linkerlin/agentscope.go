package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// mockEchoV2Agent echoes the user message in the stream (after gateway hint injection).
type mockEchoV2Agent struct{}

func (m *mockEchoV2Agent) Name() string { return "mock-echo" }

func (m *mockEchoV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(msg.GetTextContent()).Build(), nil
}

func (m *mockEchoV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent(msg.GetTextContent()).Build()
	close(ch)
	return ch, nil
}

func (m *mockEchoV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}

func (m *mockEchoV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 4)
	replyID := "echo-reply"
	text := msg.GetTextContent()
	ch <- event.NewReplyStart(replyID, m.Name())
	ch <- event.NewTextBlockDelta(replyID, 0, text)
	ch <- event.NewReplyEnd(replyID, m.Name())
	close(ch)
	return ch, nil
}

func (m *mockEchoV2Agent) LoadState(state *agent.AgentState) error { return nil }
func (m *mockEchoV2Agent) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *mockEchoV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func TestE2E_OffloadHintInjection(t *testing.T) {
	srv, toolOffload := buildGateway(&mockEchoV2Agent{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regBody := `{"name":"Bob"}`
	regResp := postJSON(t, ts.URL+"/api/v1/auth/register", regBody, "")
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %s", regResp.StatusCode, readBody(regResp))
	}
	var reg registerResponse
	decodeJSON(t, regResp, &reg)

	sessionID := "offload-e2e-session"
	toolOffload.PushResult(sessionID, "<system-notification>\nBackground task completed.\nResult:\noffload-e2e-ok\n</system-notification>")

	chatBody := `{"text":"continue","session_id":"` + sessionID + `"}`
	streamResp := postJSON(t, ts.URL+"/v2/chat/stream", chatBody, reg.APIKey)
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("chat stream: %d %s", streamResp.StatusCode, readBody(streamResp))
	}
	streamText := readBody(streamResp)
	if !strings.Contains(streamText, "offload-e2e-ok") {
		t.Fatalf("expected injected offload hint in stream, got:\n%s", streamText)
	}
}
