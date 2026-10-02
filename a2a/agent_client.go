// a2a/agent_client.go — the CLIENT direction of the agent mapping (20.5):
// a remote A2A agent (a2a.Client) becomes a LOCAL agent.Agent, so pipelines
// and workflows treat it exactly like any local agent. Acceptance clauses:
//
//   - state continues BY TASK: the remote task id returned in Message.Meta
//     rides the next request's Meta (the remote side resumes its context);
//   - a second send while a call is in flight is REJECTED (ErrAgentBusy) —
//     one remote turn at a time, no silent queueing;
//   - remote increments are NOT flattened: CallStream forwards every
//     Subscribe message as its own message.Msg (a stream stays a stream,
//     never collapsed into one final blob).
package a2a

import (
	"context"
	"errors"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// MetaTaskID is the Message.Meta key carrying the remote task id (set by the
// remote side, echoed by A2AAgent on the next turn for task continuation).
const MetaTaskID = "task_id"

// ErrAgentBusy rejects a second concurrent turn on one A2AAgent (a remote
// agent runs one task at a time per session; queueing silently would hide
// the contention).
var ErrAgentBusy = errors.New("a2a: agent busy (a turn is already in flight)")

// A2AAgent adapts a remote A2A agent into a local agent.Agent.
type A2AAgent struct {
	client Client
	name   string

	mu       sync.Mutex
	busy     bool
	lastTask string
}

// NewA2AAgent wraps client as a local agent. name labels the agent locally
// (Name()); the remote identity lives in the client configuration.
func NewA2AAgent(client Client, name string) *A2AAgent {
	if name == "" {
		name = "a2a-agent"
	}
	return &A2AAgent{client: client, name: name}
}

// Name implements agent.Agent.
func (a *A2AAgent) Name() string { return a.name }

// Call implements agent.Agent: one synchronous remote turn. The previous
// turn's remote task id (if any) rides the request Meta — state continues
// BY TASK on the remote side; the response's task id is remembered for the
// next turn.
func (a *A2AAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if err := a.enter(); err != nil {
		return nil, err
	}
	defer a.exit()

	req := a.buildRequest(msg)
	resp, err := a.client.Send(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("a2a: remote agent returned no message")
	}
	a.rememberTask(resp.Meta)
	out := message.NewMsg().Role(message.RoleAssistant).TextContent(resp.Content).Build()
	if id, ok := resp.Meta[MetaTaskID].(string); ok && id != "" {
		out.Metadata = map[string]any{MetaTaskID: id}
	}
	return out, nil
}

// CallStream implements agent.Agent: the remote increments flow through ONE
// message.Msg per Subscribe message — a stream stays a stream (never
// flattened into a final blob), and the turn is busy-guarded like Call.
func (a *A2AAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	if err := a.enter(); err != nil {
		return nil, err
	}
	req := a.buildRequest(msg)
	in, err := a.client.SendSubscribe(ctx, req)
	if err != nil {
		a.exit()
		return nil, err
	}
	out := make(chan *message.Msg, 8)
	go func() {
		defer close(out)
		defer a.exit()
		for m := range in {
			if m == nil {
				continue
			}
			a.rememberTask(m.Meta)
			b := message.NewMsg().Role(message.RoleAssistant).TextContent(m.Content).Build()
			if id, ok := m.Meta[MetaTaskID].(string); ok && id != "" {
				b.Metadata = map[string]any{MetaTaskID: id}
			}
			select {
			case out <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// buildRequest converts the local message, attaching the remembered task id
// for continuation.
func (a *A2AAgent) buildRequest(msg *message.Msg) *Message {
	req := &Message{Role: "user", Content: msg.GetTextContent()}
	a.mu.Lock()
	task := a.lastTask
	a.mu.Unlock()
	if task != "" {
		req.Meta = map[string]any{MetaTaskID: task}
	}
	return req
}

// rememberTask records the latest remote task id seen in a response.
func (a *A2AAgent) rememberTask(meta map[string]any) {
	if meta == nil {
		return
	}
	if id, ok := meta[MetaTaskID].(string); ok && id != "" {
		a.mu.Lock()
		a.lastTask = id
		a.mu.Unlock()
	}
}

func (a *A2AAgent) enter() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return ErrAgentBusy
	}
	a.busy = true
	return nil
}

func (a *A2AAgent) exit() {
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
}

var _ agent.Agent = (*A2AAgent)(nil)
