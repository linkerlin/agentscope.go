package scheduleapi

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/schedule"
)

// v2MockAgent is a configurable V2Agent for manager tests.
type v2MockAgent struct {
	events []event.AgentEvent
	delay  time.Duration
}

func (m *v2MockAgent) Name() string { return "mock" }

func (m *v2MockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *v2MockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

func (m *v2MockAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}

func (m *v2MockAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				return
			}
		}
		for _, ev := range m.events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (m *v2MockAgent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (m *v2MockAgent) LoadState(st *agent.AgentState) error  { return nil }
func (m *v2MockAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

var _ agent.Agent = (*v2MockAgent)(nil)

// nonV2Agent is a minimal agent that does not implement V2Agent.
type nonV2Agent struct{}

func (n *nonV2Agent) Name() string { return "nonv2" }
func (n *nonV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (n *nonV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

// countingAgent fails the first N calls then succeeds — retry-path probe.
type countingAgent struct {
	calls int
	fail  int
}

func (a *countingAgent) Name() string { return "counter" }

func (a *countingAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	a.calls++
	if a.calls <= a.fail {
		return nil, errors.New("boom")
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (a *countingAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	resp, err := a.Call(ctx, msg)
	ch := make(chan *message.Msg, 1)
	if err == nil {
		ch <- resp
	}
	close(ch)
	return ch, err
}

var _ agent.Agent = (*countingAgent)(nil)

// fakeAgents is an in-memory Agents registry.
type fakeAgents struct {
	mu     sync.Mutex
	agents map[string]agent.Agent
}

func (f *fakeAgents) Get(ctx context.Context, id string) (agent.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.agents[id]
	if !ok {
		return nil, errors.New("agent not found: " + id)
	}
	return a, nil
}

// fakeSessions records turns and tracks active count.
type fakeSessions struct {
	mu       sync.Mutex
	runnings map[string]int
	total    int
}

func (f *fakeSessions) Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	f.mu.Lock()
	f.runnings[sessionID]++
	f.total++
	f.mu.Unlock()

	v2, ok := a.(agent.V2Agent)
	if !ok {
		return nil, errors.New("agent does not support V2 streaming")
	}
	ch, err := v2.ReplyStream(ctx, msg)
	if err != nil {
		return nil, err
	}
	out := make(chan event.AgentEvent)
	go func() {
		defer close(out)
		for ev := range ch {
			out <- ev
		}
		f.mu.Lock()
		f.runnings[sessionID]--
		f.mu.Unlock()
	}()
	return out, nil
}

func (f *fakeSessions) ActiveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, v := range f.runnings {
		n += v
	}
	return n
}

func TestBackgroundTaskManager_ScheduleAndCancel(t *testing.T) {
	reg := &fakeAgents{agents: map[string]agent.Agent{
		"a1": &v2MockAgent{events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		}},
	}}

	btm := NewBackgroundTaskManager(reg, nil)
	btm.Start()
	defer btm.Stop()

	job := &schedule.Job{
		ID:       "j1",
		AgentID:  "a1",
		CronExpr: "*/1 * * * *",
		Payload:  "hello",
	}
	if err := btm.Schedule(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	next, err := btm.NextRun("j1")
	if err != nil {
		t.Fatal(err)
	}
	if next.IsZero() {
		t.Fatal("expected non-zero next run")
	}

	if err := btm.Cancel(context.Background(), "j1"); err != nil {
		t.Fatal(err)
	}

	_, err = btm.NextRun("j1")
	if err == nil {
		t.Fatal("expected error after cancel")
	}
}

func TestBackgroundTaskManager_HandleAgentNotFound(t *testing.T) {
	reg := &fakeAgents{}
	btm := NewBackgroundTaskManager(reg, nil)

	job := &schedule.Job{
		ID:      "j1",
		AgentID: "missing",
		Payload: "hello",
	}
	err := btm.handle(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestBackgroundTaskManager_HandleWithSessionManager(t *testing.T) {
	reg := &fakeAgents{agents: map[string]agent.Agent{
		"a1": &v2MockAgent{events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		}, delay: 10 * time.Millisecond},
	}}

	sm := &fakeSessions{runnings: map[string]int{}}
	btm := NewBackgroundTaskManager(reg, sm)

	job := &schedule.Job{
		ID:        "j1",
		AgentID:   "a1",
		SessionID: "s1",
		Payload:   "hello",
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	if sm.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions after handle, got %d", sm.ActiveCount())
	}
}

func TestBackgroundTaskManager_HandleV2Agent(t *testing.T) {
	reg := &fakeAgents{agents: map[string]agent.Agent{
		"a1": &v2MockAgent{events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		}},
	}}

	btm := NewBackgroundTaskManager(reg, nil)
	job := &schedule.Job{
		ID:      "j1",
		AgentID: "a1",
		Payload: "hello",
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundTaskManager_HandleNonV2Agent(t *testing.T) {
	reg := &fakeAgents{agents: map[string]agent.Agent{"a1": &nonV2Agent{}}}

	btm := NewBackgroundTaskManager(reg, nil)
	job := &schedule.Job{
		ID:      "j1",
		AgentID: "a1",
		Payload: "hello",
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundTaskManager_RetryOnFailure(t *testing.T) {
	ag := &countingAgent{fail: 2}
	reg := &fakeAgents{agents: map[string]agent.Agent{"counter": ag}}

	btm := NewBackgroundTaskManager(reg, nil)
	job := &schedule.Job{
		ID:         "j1",
		AgentID:    "counter",
		Payload:    "hi",
		MaxRetries: 3,
		RetryDelay: 5 * time.Millisecond,
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatalf("expected success after retries, got %v (calls=%d)", err, ag.calls)
	}
	if ag.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", ag.calls)
	}
}
