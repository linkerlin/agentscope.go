package channel

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockChannel is an in-memory Channel for tests. Start feeds events from a
// queue into emit; SendText records the output.
type mockChannel struct {
	id      string
	started chan struct{}
	emitted []ChannelEvent
	sent    []string
	mu      sync.Mutex
	stop    chan struct{}
}

func newMockChannel(id string) *mockChannel {
	return &mockChannel{
		id:      id,
		started: make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}
}

func (m *mockChannel) ID() string { return m.id }

func (m *mockChannel) Start(ctx context.Context, emit func(ChannelEvent) error) error {
	m.started <- struct{}{}
	<-ctx.Done()
	return nil
}

func (m *mockChannel) SendText(ctx context.Context, chatID, text string) error {
	m.mu.Lock()
	m.sent = append(m.sent, text)
	m.mu.Unlock()
	return nil
}

func (m *mockChannel) Close() error { return nil }

func (m *mockChannel) push(ev ChannelEvent) {
	m.mu.Lock()
	m.emitted = append(m.emitted, ev)
	m.mu.Unlock()
}

// --- stub router/runner ---

type stubRouter struct{ agentID, sessionID string }

func (s stubRouter) Resolve(ctx context.Context, ev ChannelEvent) (string, string, error) {
	return s.agentID, s.sessionID, nil
}

type stubRunner struct {
	mu        sync.Mutex
	turns     []ChannelEvent
	lastAgent string
}

func (s *stubRunner) RunUserTurn(ctx context.Context, agentID, sessionID string, ev ChannelEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turns = append(s.turns, ev)
	s.lastAgent = agentID
	return nil
}

func TestGateway_HandlesEvent(t *testing.T) {
	r := stubRouter{agentID: "a1", sessionID: "s1"}
	runner := &stubRunner{}
	g := NewGateway(r, runner)

	ev := ChannelEvent{ChannelID: "c1", ChatID: "chat-1", Text: "hello"}
	if err := g.HandleEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(runner.turns) != 1 || runner.turns[0].Text != "hello" {
		t.Fatalf("runner turns: %+v", runner.turns)
	}
	if runner.lastAgent != "a1" {
		t.Fatalf("agent = %q", runner.lastAgent)
	}
}

func TestGateway_IgnoresEmptyChat(t *testing.T) {
	g := NewGateway(stubRouter{}, &stubRunner{})
	if err := g.HandleEvent(context.Background(), ChannelEvent{ChannelID: "c1"}); err != nil {
		t.Fatalf("empty chat should be ignored, got %v", err)
	}
}

func TestGateway_MissingRouter(t *testing.T) {
	g := NewGateway(nil, &stubRunner{})
	if err := g.HandleEvent(context.Background(), ChannelEvent{ChannelID: "c", ChatID: "x"}); err == nil {
		t.Fatal("expected error for missing router")
	}
}

func TestRegistry_CRUD(t *testing.T) {
	reg := NewRegistry()
	c := newMockChannel("discord-1")
	if err := reg.Register(c); err != nil {
		t.Fatal(err)
	}
	if reg.Get("discord-1") != c {
		t.Fatal("get failed")
	}
	if len(reg.List()) != 1 {
		t.Fatal("list failed")
	}
	reg.Remove("discord-1")
	if reg.Get("discord-1") != nil {
		t.Fatal("remove failed")
	}
}

func TestRegistry_InvalidChannel(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(&mockChannel{id: ""}); err == nil {
		t.Fatal("expected error for empty channel id")
	}
}

func TestDispatcher_StartsChannels(t *testing.T) {
	reg := NewRegistry()
	c := newMockChannel("c1")
	reg.Register(c)
	g := NewGateway(stubRouter{}, &stubRunner{})
	d := NewDispatcher(g, reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.StartAll(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.started:
		// started
	case <-time.After(time.Second):
		t.Fatal("channel not started")
	}
}

func TestDispatcher_MissingGateway(t *testing.T) {
	d := NewDispatcher(nil, NewRegistry())
	if err := d.StartAll(context.Background()); err == nil {
		t.Fatal("expected error for missing gateway")
	}
}

// --- routing tests ---

func TestRouteTable_ExactMatchWins(t *testing.T) {
	table := RouteTable{ChannelID: "c1", Bindings: []Binding{
		{ChatIDPrefix: "dev-", AgentID: "prefix-agent", SessionPrefix: "dev-"},
		{ChatID: "dev-123", AgentID: "exact-agent", SessionPrefix: "ex-"},
	}}
	agent, sid, err := table.Resolve(context.Background(), ChannelEvent{ChannelID: "c1", ChatID: "dev-123"})
	if err != nil {
		t.Fatal(err)
	}
	if agent != "exact-agent" || sid != "ex-dev-123" {
		t.Fatalf("got agent=%q sid=%q", agent, sid)
	}
}

func TestRouteTable_PrefixMatch(t *testing.T) {
	table := RouteTable{ChannelID: "c1", Bindings: []Binding{
		{ChatIDPrefix: "dev-", AgentID: "dev-agent", SessionPrefix: "dev-"},
		{ChatIDPrefix: "qa-", AgentID: "qa-agent", SessionPrefix: "qa-"},
	}}
	agent, _, _ := table.Resolve(context.Background(), ChannelEvent{ChannelID: "c1", ChatID: "qa-42"})
	if agent != "qa-agent" {
		t.Fatalf("got %q want qa-agent", agent)
	}
}

func TestRouteTable_DefaultBinding(t *testing.T) {
	table := RouteTable{ChannelID: "c1", Bindings: []Binding{
		{AgentID: "default-agent", SessionPrefix: "d-"},
	}}
	agent, sid, _ := table.Resolve(context.Background(), ChannelEvent{ChannelID: "c1", ChatID: "anything"})
	if agent != "default-agent" || sid != "d-anything" {
		t.Fatalf("got agent=%q sid=%q", agent, sid)
	}
}

func TestRouteTable_NoMatch(t *testing.T) {
	table := RouteTable{ChannelID: "c1", Bindings: []Binding{
		{ChatIDPrefix: "dev-", AgentID: "a", SessionPrefix: "d-"},
	}}
	if _, _, err := table.Resolve(context.Background(), ChannelEvent{ChannelID: "c1", ChatID: "other"}); err == nil {
		t.Fatal("expected no-match error")
	}
}

func TestChatRouter_ChannelFallback(t *testing.T) {
	r := NewChatRouter(
		RouteTable{ChannelID: "c1", Bindings: []Binding{{ChatIDPrefix: "x-", AgentID: "xa", SessionPrefix: ""}}},
	)
	// event for c2 falls back to default table
	r.AddTable(RouteTable{ChannelID: "", Bindings: []Binding{{AgentID: "default", SessionPrefix: ""}}})
	agent, _, _ := r.Resolve(context.Background(), ChannelEvent{ChannelID: "c2", ChatID: "y-1"})
	if agent != "default" {
		t.Fatalf("got %q want default", agent)
	}
}

func TestChatRouter_NoTable(t *testing.T) {
	r := NewChatRouter()
	if _, _, err := r.Resolve(context.Background(), ChannelEvent{ChannelID: "x", ChatID: "y"}); err == nil {
		t.Fatal("expected error for missing table")
	}
}
