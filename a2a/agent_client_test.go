// a2a/agent_client_test.go — 20.5 acceptance: the remote agent maps to a
// local agent.Agent, state continues by task, a second in-flight send is
// rejected, and remote increments are never flattened. Plus the
// ClusterManager fix: real (injected) clients, no fabricated completions,
// failover re-routes to other nodes.
package a2a

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// ---- fakes ----

type fakeRemote struct {
	mu sync.Mutex
	// sends records every request Content+Meta this remote saw.
	sends []Message
	// replyFor: first Send returns task t1, second returns task t2 (task
	// continuation is observable).
	replySeq []Message
	// streamMsgs are returned by SendSubscribe.
	streamMsgs []Message
	// blockUntil, when set, makes Send wait (busy-rejection tests).
	block chan struct{}

	sendErr error
	sendNil bool
}

func (f *fakeRemote) Send(ctx context.Context, msg *Message) (*Message, error) {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.mu.Lock()
	f.sends = append(f.sends, *msg)
	idx := len(f.sends) - 1
	f.mu.Unlock()
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	if f.sendNil {
		return nil, nil
	}
	if idx < len(f.replySeq) {
		resp := f.replySeq[idx]
		return &resp, nil
	}
	return &Message{Role: "agent", Content: "ok"}, nil
}

func (f *fakeRemote) SendSubscribe(ctx context.Context, msg *Message) (<-chan *Message, error) {
	f.mu.Lock()
	f.sends = append(f.sends, *msg)
	f.mu.Unlock()
	out := make(chan *Message, len(f.streamMsgs)+1)
	for i := range f.streamMsgs {
		out <- &f.streamMsgs[i]
	}
	close(out)
	return out, nil
}

func (f *fakeRemote) Close() error { return nil }

func (f *fakeRemote) lastSend() Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sends) == 0 {
		return Message{}
	}
	return f.sends[len(f.sends)-1]
}

// ---- A2AAgent ----

// TestA2AAgent_CallAndTaskContinuation: the remote agent behaves as a local
// agent.Agent, and the remote task id rides the NEXT request's Meta — state
// continues by task.
func TestA2AAgent_CallAndTaskContinuation(t *testing.T) {
	remote := &fakeRemote{replySeq: []Message{
		{Role: "agent", Content: "第一轮", Meta: map[string]any{MetaTaskID: "task-1"}},
		{Role: "agent", Content: "第二轮", Meta: map[string]any{MetaTaskID: "task-2"}},
	}}
	ag := NewA2AAgent(remote, "remote-1")

	if ag.Name() != "remote-1" {
		t.Fatalf("name: %s", ag.Name())
	}
	m1, err := ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if m1.GetTextContent() != "第一轮" || m1.Metadata[MetaTaskID] != "task-1" {
		t.Fatalf("turn 1: %+v", m1)
	}

	m2, err := ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("next").Build())
	if err != nil {
		t.Fatal(err)
	}
	if m2.Metadata[MetaTaskID] != "task-2" {
		t.Fatalf("turn 2: %+v", m2)
	}
	// The second REQUEST carried the first turn's task id.
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if len(remote.sends) != 2 {
		t.Fatalf("sends: %d", len(remote.sends))
	}
	if remote.sends[1].Meta[MetaTaskID] != "task-1" {
		t.Fatalf("task continuation missing: %+v", remote.sends[1].Meta)
	}
	if remote.sends[0].Meta != nil {
		t.Fatalf("first send must carry no task id: %+v", remote.sends[0].Meta)
	}
}

// TestA2AAgent_BusyRejectsSecondSend: while one turn is in flight, a second
// send is rejected with ErrAgentBusy — both on Call and CallStream.
func TestA2AAgent_BusyRejectsSecondSend(t *testing.T) {
	unblock := make(chan struct{})
	remote := &fakeRemote{block: unblock}
	ag := NewA2AAgent(remote, "r")

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("slow").Build())
		if err != nil {
			t.Errorf("first call: %v", err)
		}
	}()
	// Let the first call enter the remote (blocks there).
	time.Sleep(50 * time.Millisecond)

	if _, err := ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("second").Build()); !errors.Is(err, ErrAgentBusy) {
		t.Fatalf("second call must be ErrAgentBusy, got %v", err)
	}
	if _, err := ag.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("second").Build()); !errors.Is(err, ErrAgentBusy) {
		t.Fatalf("second stream must be ErrAgentBusy, got %v", err)
	}

	close(unblock)
	<-done
	// After the turn completes, sends are accepted again.
	if _, err := ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("after").Build()); err != nil {
		t.Fatalf("post-turn call: %v", err)
	}
}

// TestA2AAgent_StreamNotFlattened: every Subscribe message becomes its own
// message.Msg — remote increments stay a stream, never a final blob.
func TestA2AAgent_StreamNotFlattened(t *testing.T) {
	remote := &fakeRemote{streamMsgs: []Message{
		{Role: "agent", Content: "第一"},
		{Role: "agent", Content: "段"},
		{Role: "agent", Content: "。", Meta: map[string]any{MetaTaskID: "task-s"}},
	}}
	ag := NewA2AAgent(remote, "r")

	ch, err := ag.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	var lastTask any
	for m := range ch {
		parts = append(parts, m.GetTextContent())
		if m.Metadata != nil {
			lastTask = m.Metadata[MetaTaskID]
		}
	}
	if len(parts) != 3 || strings.Join(parts, "") != "第一段。" {
		t.Fatalf("stream parts: %v", parts)
	}
	if lastTask != "task-s" {
		t.Fatalf("stream task id: %v", lastTask)
	}
	// The next request continues from the streamed task id.
	_, _ = ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("again").Build())
	if got := remote.lastSend().Meta[MetaTaskID]; got != "task-s" {
		t.Fatalf("continuation after stream: %v", got)
	}
}

// TestA2AAgent_RemoteFailures: remote errors and empty replies surface as
// local errors (no fabricated success).
func TestA2AAgent_RemoteFailures(t *testing.T) {
	ag := NewA2AAgent(&fakeRemote{sendErr: errors.New("remote down")}, "r")
	if _, err := ag.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build()); err == nil || !strings.Contains(err.Error(), "remote down") {
		t.Fatalf("send error: %v", err)
	}

	agNil := NewA2AAgent(&fakeRemote{sendNil: true}, "r")
	if _, err := agNil.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build()); err == nil || !strings.Contains(err.Error(), "no message") {
		t.Fatalf("nil reply must error: %v", err)
	}
}

// ---- ClusterManager (20.5 fix) ----

type scriptedNodeClient struct {
	err    error
	nilRep bool
	agent  string
}

func (c *scriptedNodeClient) Send(ctx context.Context, msg *Message) (*Message, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.nilRep {
		return nil, nil
	}
	return &Message{Role: "agent", Content: "from-" + c.agent, Meta: map[string]any{"status": "completed"}}, nil
}
func (c *scriptedNodeClient) SendSubscribe(ctx context.Context, msg *Message) (<-chan *Message, error) {
	return nil, errors.New("unused")
}
func (c *scriptedNodeClient) Close() error { return nil }

// TestClusterManager_RealClientAndFailover: sendTaskToNode uses the injected
// (real-shape) clients; a failing node re-routes to a healthy one; a nil
// reply is an ERROR, never a fabricated completed result.
func TestClusterManager_RealClientAndFailover(t *testing.T) {
	reg := NewRegistry()
	cm := NewClusterManager(reg, "local", "http://local")
	cm.SetFailoverTimeout(time.Millisecond)
	cm.SetMaxRetries(3)

	failClient := &scriptedNodeClient{err: errors.New("node down")}
	okClient := &scriptedNodeClient{agent: "node2"}
	nilClient := &scriptedNodeClient{nilRep: true}
	cm.WithClientFactory(func(url string) Client {
		switch {
		case strings.Contains(url, "bad"):
			return failClient
		case strings.Contains(url, "empty"):
			return nilClient
		default:
			return okClient
		}
	})

	// Seed node health directly (no live health-check loop needed).
	cm.mu.Lock()
	cm.nodeHealth = map[string]*NodeHealth{
		"http://bad-node":   {URL: "http://bad-node", Healthy: true, Load: 0.1},
		"http://empty-node": {URL: "http://empty-node", Healthy: true, Load: 0.1},
		"http://good-node":  {URL: "http://good-node", Healthy: true, Load: 0.1},
	}
	cm.mu.Unlock()

	// Failover: routing to the bad node re-routes to a good one.
	result, err := cm.SendTaskWithFailover(context.Background(), &Task{ID: "t1"})
	if err != nil {
		t.Fatalf("failover send: %v", err)
	}
	if result.Output != "from-node2" || result.NodeID != "http://good-node" {
		t.Fatalf("result: %+v", result)
	}

	// A node that answers NOTHING fails the task (no fabricated
	// "completed").
	cm.mu.Lock()
	for _, h := range cm.nodeHealth {
		h.Healthy = false
	}
	cm.nodeHealth = map[string]*NodeHealth{
		"http://empty-node": {URL: "http://empty-node", Healthy: true, Load: 0.1},
	}
	cm.mu.Unlock()
	if _, err := cm.SendTaskWithFailover(context.Background(), &Task{ID: "t2"}); err == nil {
		t.Fatal("a nil reply must fail the task, not fabricate completion")
	}
}
