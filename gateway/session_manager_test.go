package gateway

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

// smMockAgent is a configurable V2Agent for SessionManager tests.
type smMockAgent struct {
	events []event.AgentEvent
	delay  time.Duration
}

func (m *smMockAgent) Name() string { return "mock" }

func (m *smMockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *smMockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

func (m *smMockAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}

func (m *smMockAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			time.Sleep(m.delay)
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

func (m *smMockAgent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (m *smMockAgent) LoadState(st *agent.AgentState) error  { return nil }
func (m *smMockAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func makeMockAgent(events []event.AgentEvent, delay time.Duration) agent.Agent {
	return &smMockAgent{events: events, delay: delay}
}

func TestSessionManager_BasicRun(t *testing.T) {
	sm := NewSessionManager()
	evts := []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewTextBlockDelta("r1", 0, "hello"),
		event.NewReplyEnd("r1", "mock"),
	}
	a := makeMockAgent(evts, 0)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	ch, err := sm.Run(context.Background(), "s1", a, msg)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for ev := range ch {
		got = append(got, ev.EventType())
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(got), got)
	}
	if got[0] != "reply_start" || got[1] != "text_block_delta" || got[2] != "reply_end" {
		t.Fatalf("unexpected events: %v", got)
	}
}

func TestSessionManager_Serialisation(t *testing.T) {
	sm := NewSessionManager()
	// First agent produces 3 events with a small delay.
	a1 := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewTextBlockDelta("r1", 0, "first"),
		event.NewReplyEnd("r1", "mock"),
	}, 20*time.Millisecond)
	// Second agent produces 2 events.
	a2 := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r2", "mock"),
		event.NewReplyEnd("r2", "mock"),
	}, 0)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	started := make(chan struct{})
	go func() {
		defer wg.Done()
		ch, _ := sm.Run(context.Background(), "s1", a1, msg)
		close(started)
		for range ch {
		}
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		<-started // wait until the first run has actually acquired the lock
		ch, _ := sm.Run(context.Background(), "s1", a2, msg)
		for range ch {
		}
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	}()

	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("expected serial order [1,2], got %v", order)
	}
}

func TestSessionManager_DifferentSessionsParallel(t *testing.T) {
	sm := NewSessionManager()
	a1 := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewReplyEnd("r1", "mock"),
	}, 50*time.Millisecond)
	a2 := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r2", "mock"),
		event.NewReplyEnd("r2", "mock"),
	}, 50*time.Millisecond)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	var start1, end1, start2, end2 time.Time
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		start1 = time.Now()
		ch, _ := sm.Run(context.Background(), "s1", a1, msg)
		for range ch {
		}
		end1 = time.Now()
	}()

	go func() {
		defer wg.Done()
		start2 = time.Now()
		ch, _ := sm.Run(context.Background(), "s2", a2, msg)
		for range ch {
		}
		end2 = time.Now()
	}()

	wg.Wait()

	// Different sessions should run in parallel (overlap).
	if start2.After(end1) || start1.After(end2) {
		t.Fatal("expected parallel execution for different sessions")
	}
}

func TestSessionManager_SubscribeReplay(t *testing.T) {
	sm := NewSessionManager()
	evts := []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewTextBlockDelta("r1", 0, "hello"),
	}
	a := makeMockAgent(evts, 20*time.Millisecond)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	// Start the run.
	ch1, _ := sm.Run(context.Background(), "s1", a, msg)
	// Wait for first two events to be produced.
	time.Sleep(10 * time.Millisecond)

	// Late-joining subscriber.
	ch2 := sm.Subscribe("s1")

	// Collect from ch1.
	var fromFirst []string
	for ev := range ch1 {
		fromFirst = append(fromFirst, ev.EventType())
	}

	// Collect from ch2 (should have replay + end event).
	var fromSecond []string
	for ev := range ch2 {
		fromSecond = append(fromSecond, ev.EventType())
	}

	// ch2 should replay the first two events and also get the end event
	// because it subscribed before the run finished.
	if len(fromSecond) < 2 {
		t.Fatalf("expected at least 2 replay events in second subscriber, got %d: %v", len(fromSecond), fromSecond)
	}
	if fromSecond[0] != "reply_start" {
		t.Fatalf("expected replay start, got %v", fromSecond)
	}
}

func TestSessionManager_FanOut(t *testing.T) {
	sm := NewSessionManager()
	evts := []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewTextBlockDelta("r1", 0, "hello"),
		event.NewReplyEnd("r1", "mock"),
	}
	a := makeMockAgent(evts, 0)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	ch1, _ := sm.Run(context.Background(), "s1", a, msg)
	ch2 := sm.Subscribe("s1")

	var count1, count2 int32
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range ch1 {
			atomic.AddInt32(&count1, 1)
		}
	}()
	go func() {
		defer wg.Done()
		for range ch2 {
			atomic.AddInt32(&count2, 1)
		}
	}()

	wg.Wait()

	if count1 != 3 {
		t.Fatalf("expected count1=3, got %d", count1)
	}
	if count2 != 3 {
		t.Fatalf("expected count2=3, got %d", count2)
	}
}

// nonV2Agent is a minimal agent that does not implement V2Agent.
type nonV2Agent struct{}

func (n *nonV2Agent) Name() string { return "nonv2" }
func (n *nonV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (n *nonV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func TestSessionManager_NonV2Agent(t *testing.T) {
	sm := NewSessionManager()
	_, err := sm.Run(context.Background(), "s1", &nonV2Agent{}, message.NewMsg().Build())
	if err == nil || !strings.Contains(err.Error(), "does not support V2") {
		t.Fatalf("expected V2 error, got: %v", err)
	}
}

func TestSessionManager_SubscribeAfterDone(t *testing.T) {
	sm := NewSessionManager()
	evts := []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewReplyEnd("r1", "mock"),
	}
	a := makeMockAgent(evts, 0)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	ch1, _ := sm.Run(context.Background(), "s1", a, msg)
	for range ch1 {
	} // wait for completion

	// Subscribe after run is done.
	ch2 := sm.Subscribe("s1")
	var got []string
	for ev := range ch2 {
		got = append(got, ev.EventType())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 replay events after done, got %d", len(got))
	}
}

func TestSessionManager_IsActive(t *testing.T) {
	sm := NewSessionManager()
	a := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewReplyEnd("r1", "mock"),
	}, 50*time.Millisecond)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	if sm.IsActive("s1") {
		t.Fatal("expected s1 not active before run")
	}

	ch, _ := sm.Run(context.Background(), "s1", a, msg)
	if !sm.IsActive("s1") {
		t.Fatal("expected s1 active during run")
	}

	for range ch {
	}

	if sm.IsActive("s1") {
		t.Fatal("expected s1 not active after run")
	}
}

func TestSessionManager_ActiveCount(t *testing.T) {
	sm := NewSessionManager()
	a1 := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewReplyEnd("r1", "mock"),
	}, 50*time.Millisecond)
	a2 := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r2", "mock"),
		event.NewReplyEnd("r2", "mock"),
	}, 50*time.Millisecond)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	if sm.ActiveCount() != 0 {
		t.Fatalf("expected 0 active, got %d", sm.ActiveCount())
	}

	ch1, _ := sm.Run(context.Background(), "s1", a1, msg)
	ch2, _ := sm.Run(context.Background(), "s2", a2, msg)

	if sm.ActiveCount() != 2 {
		t.Fatalf("expected 2 active, got %d", sm.ActiveCount())
	}

	for range ch1 {
	}
	for range ch2 {
	}

	if sm.ActiveCount() != 0 {
		t.Fatalf("expected 0 active after completion, got %d", sm.ActiveCount())
	}
}

// trackingStorage is a minimal Storage that records UpsertMessage calls.
type trackingStorage struct {
	service.MemoryStorage
	upserts []*service.StoredMessage
}

func (t *trackingStorage) UpsertMessage(ctx context.Context, msg *service.StoredMessage) error {
	t.upserts = append(t.upserts, msg)
	return t.MemoryStorage.UpsertMessage(ctx, msg)
}

func TestSessionManager_WithStorage_PersistsMessages(t *testing.T) {
	store := &trackingStorage{MemoryStorage: *service.NewMemoryStorage()}
	sm := NewSessionManager().WithStorage(store)

	evts := []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewTextBlockStart("r1", 0),
		event.NewTextBlockDelta("r1", 0, "hello"),
		event.NewTextBlockEnd("r1", 0),
		event.NewReplyEnd("r1", "mock"),
	}
	a := makeMockAgent(evts, 0)

	inputMsg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	ch, err := sm.Run(context.Background(), "s1", a, inputMsg)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	} // drain

	// Should have upserted both input and reply messages.
	if len(store.upserts) != 2 {
		t.Fatalf("expected 2 upserts (input + reply), got %d", len(store.upserts))
	}

	// Verify input message.
	inputStored := store.upserts[0]
	if inputStored.Role != "user" || inputStored.Content != "hi" {
		t.Fatalf("unexpected input stored: %+v", inputStored)
	}

	// Verify reply message was reconstructed from event stream.
	replyStored := store.upserts[1]
	if replyStored.Role != "assistant" {
		t.Fatalf("expected assistant role, got %s", replyStored.Role)
	}
	if replyStored.Content != "hello" {
		t.Fatalf("expected reply content 'hello', got %s", replyStored.Content)
	}
	if replyStored.FinishedAt == nil {
		t.Fatal("expected FinishedAt to be set on reply")
	}
}

func TestSessionManager_WithStorage_PersistsToolCall(t *testing.T) {
	store := &trackingStorage{MemoryStorage: *service.NewMemoryStorage()}
	sm := NewSessionManager().WithStorage(store)

	evts := []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewToolCallStart("r1", 0, "tc1", "calc"),
		event.NewToolCallDelta("r1", 0, "tc1", `{"expr":"1+1"}`),
		event.NewToolCallEnd("r1", 0, "tc1"),
		event.NewReplyEnd("r1", "mock"),
	}
	a := makeMockAgent(evts, 0)

	ch, err := sm.Run(context.Background(), "s1", a, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}

	// Reply message should have tool_use block in Blocks.
	replyStored := store.upserts[0]
	if replyStored.Blocks == "" {
		t.Fatal("expected Blocks to contain tool_use block")
	}
}

func TestSessionManager_Terminate(t *testing.T) {
	sm := NewSessionManager()
	a := makeMockAgent([]event.AgentEvent{
		event.NewTextBlockDelta("r1", 0, "slow"),
		event.NewReplyEnd("r1", ""),
	}, 500*time.Millisecond)

	ch, err := sm.Run(context.Background(), "sess-term", a, message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}

	if !sm.IsActive("sess-term") {
		t.Fatal("expected active run before terminate")
	}
	if !sm.Terminate("sess-term") {
		t.Fatal("expected terminate to succeed")
	}

	deadline := time.After(2 * time.Second)
	for sm.IsActive("sess-term") {
		select {
		case <-deadline:
			t.Fatal("run did not finish after terminate")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	for range ch {
	}

	if sm.Terminate("sess-term") {
		t.Fatal("terminate after run ended should be no-op")
	}
}
