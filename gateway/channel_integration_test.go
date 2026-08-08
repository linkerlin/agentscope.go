package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// mockChannel records sends for assertion.
type mockChannelRecorder struct {
	mu   sync.Mutex
	sent []string
}

func (m *mockChannelRecorder) SendText(ctx context.Context, chatID, text string) error {
	m.mu.Lock()
	m.sent = append(m.sent, text)
	m.mu.Unlock()
	return nil
}

func TestChannelRunner_RunsAgentAndReplies(t *testing.T) {
	reg := NewAgentRegistry()
	// register a simple agent that echoes "reply: <text>"
	reg.Register("agent-1", &echoAgent{})
	sessions := NewSessionManager()
	rec := &mockChannelRecorder{}

	runner := NewChannelRunner(reg, sessions).
		WithLookup(func(channelID string) channel.Channel {
			return webhookAdapter{send: rec.SendText}
		})

	err := runner.RunUserTurn(context.Background(), "agent-1", "session-1",
		channel.ChannelEvent{ChannelID: "wh-1", ChatID: "chat-1", Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	// wait for the async run + reply
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec.mu.Lock()
		n := len(rec.sent)
		rec.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.sent) == 0 {
		t.Fatal("no reply sent")
	}
	if !strings.Contains(rec.sent[0], "reply: hello") {
		t.Fatalf("reply = %q", rec.sent[0])
	}
}

func TestChannelRunner_DropsInflight(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("agent-1", &echoAgent{})
	sessions := NewSessionManager()
	runner := NewChannelRunner(reg, sessions)

	// first turn starts
	err := runner.RunUserTurn(context.Background(), "agent-1", "s1",
		channel.ChannelEvent{ChannelID: "w", ChatID: "c", Text: "one"})
	if err != nil {
		t.Fatal(err)
	}
	// second turn on same session should be dropped (no duplicate run)
	time.Sleep(20 * time.Millisecond)
	err = runner.RunUserTurn(context.Background(), "agent-1", "s1",
		channel.ChannelEvent{ChannelID: "w", ChatID: "c", Text: "two"})
	if err != nil {
		t.Fatal(err)
	}
	// wait a bit; no panic = ok
	time.Sleep(50 * time.Millisecond)
}

func TestChannelRunner_MissingAgent(t *testing.T) {
	reg := NewAgentRegistry()
	runner := NewChannelRunner(reg, NewSessionManager())
	err := runner.RunUserTurn(context.Background(), "ghost", "s1",
		channel.ChannelEvent{ChannelID: "w", ChatID: "c", Text: "hi"})
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestChannelRunner_NilDepsNoop(t *testing.T) {
	runner := NewChannelRunner(nil, nil)
	if err := runner.RunUserTurn(context.Background(), "a", "s", channel.ChannelEvent{Text: "x"}); err != nil {
		t.Fatal("nil deps should no-op")
	}
}

func TestChannelWebhookRoute_Delivers(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("dev-agent", &echoAgent{})
	sessions := NewSessionManager()

	rec := &mockChannelRecorder{}
	wh := channel.NewWebhookChannel("webhook-1")

	// build the full channel subsystem
	router := channel.NewChatRouter(channel.RouteTable{
		ChannelID: "webhook-1",
		Bindings:  []channel.Binding{{ChatIDPrefix: "dev-", AgentID: "dev-agent", SessionPrefix: "dev-"}},
	})
	runner := NewChannelRunner(reg, sessions).WithLookup(func(channelID string) channel.Channel {
		return webhookAdapter{send: rec.SendText}
	})
	gw := channel.NewGateway(router, runner)

	srv := NewServer(&mockAgent{})
	srv.WithChannelGateway(channel.NewRegistry(), gw)
	srv.RegisterChannelRoutes()
	// register the webhook instance into the registry used by the handler
	channelRegistry := srv.channelRegistry
	channelRegistry.Register(wh)
	// start the webhook channel so its emit callback is wired
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	if err := wh.Start(startCtx, func(ev channel.ChannelEvent) error {
		return gw.HandleEvent(context.Background(), ev)
	}); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewReader([]byte(`{"chat_id":"dev-9","user_id":"u1","text":"ping"}`))
	req := httptest.NewRequest("POST", "/api/v1/channels/webhook-1/webhook", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook route status = %d %s", w.Code, w.Body.String())
	}

	// wait for reply
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec.mu.Lock()
		n := len(rec.sent)
		rec.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.sent) == 0 {
		t.Fatal("no reply sent through webhook loop")
	}
	if !strings.Contains(rec.sent[0], "reply: ping") {
		t.Fatalf("reply = %q", rec.sent[0])
	}
}

func TestChannelListRoute(t *testing.T) {
	srv := NewServer(&mockAgent{})
	reg := channel.NewRegistry()
	reg.Register(channel.NewWebhookChannel("wh-1"))
	reg.Register(channel.NewWebhookChannel("wh-2"))
	srv.WithChannelGateway(reg, channel.NewGateway(stubChatRouter{}, &stubRunnerAdapter{}))
	srv.RegisterChannelRoutes()

	req := httptest.NewRequest("GET", "/api/v1/channels", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var resp struct {
		Channels []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"channels"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}
}

func TestChannelWebhookRoute_NotFound(t *testing.T) {
	srv := NewServer(&mockAgent{})
	reg := channel.NewRegistry()
	srv.WithChannelGateway(reg, channel.NewGateway(stubChatRouter{}, &stubRunnerAdapter{}))
	srv.RegisterChannelRoutes()

	req := httptest.NewRequest("POST", "/api/v1/channels/ghost/webhook", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- test helpers ---

// echoAgent is a V2Agent that emits a reply text event stream.
type echoAgent struct{}

func (e *echoAgent) Name() string { return "echo" }
func (e *echoAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).
		TextContent("reply: " + msg.GetTextContent()).Build(), nil
}
func (e *echoAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("reply: " + msg.GetTextContent()).Build()
	close(ch)
	return ch, nil
}
func (e *echoAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 4)
	text := msg.GetTextContent()
	go func() {
		ch <- event.NewReplyStart("r1", e.Name())
		ch <- event.NewTextBlockStart("r1", 0)
		ch <- event.NewTextBlockDelta("r1", 0, "reply: "+text)
		ch <- event.NewTextBlockEnd("r1", 0)
		ch <- event.NewReplyEnd("r1", e.Name())
		close(ch)
	}()
	return ch, nil
}

// Reply is the synchronous counterpart to ReplyStream (V2Agent contract).
func (e *echoAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).
		TextContent("reply: " + msg.GetTextContent()).Build(), nil
}

// LoadState/SaveState are no-ops for the stateless echo agent.
func (e *echoAgent) LoadState(state *agent.AgentState) error { return nil }
func (e *echoAgent) SaveState() (*agent.AgentState, error)   { return nil, nil }

// InjectEvent is a no-op (the echo agent never suspends).
func (e *echoAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error { return nil }

var _ agent.V2Agent = (*echoAgent)(nil)

// webhookAdapter adapts a webhook channel for SendText recording.
type webhookAdapter struct {
	send func(ctx context.Context, chatID, text string) error
}

func (w webhookAdapter) ID() string { return "wh-adapter" }
func (w webhookAdapter) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
	return nil
}
func (w webhookAdapter) SendText(ctx context.Context, chatID, text string) error {
	return w.send(ctx, chatID, text)
}
func (w webhookAdapter) Close() error { return nil }

// stubChatRouter always resolves to a fixed agent/session.
type stubChatRouter struct{}

func (stubChatRouter) Resolve(ctx context.Context, ev channel.ChannelEvent) (string, string, error) {
	return "agent-x", "session-x", nil
}

type stubRunnerAdapter struct{}

func (stubRunnerAdapter) RunUserTurn(ctx context.Context, agentID, sessionID string, ev channel.ChannelEvent) error {
	return nil
}
