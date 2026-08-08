package gateway

import (
	"context"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// ChannelRunner adapts the gateway's AgentRegistry + SessionManager into a
// channel.Runner. It runs the resolved agent on the derived session and, when
// the run finishes, sends the final text reply back to the originating chat via
// the owning channel (mirroring Python's "output flows back through the
// dispatcher" design).
type ChannelRunner struct {
	Registry *AgentRegistry
	Sessions *SessionManager
	// Lookup returns the channel instance owning the given channel id.
	Lookup func(channelID string) channel.Channel

	mu     sync.Mutex
	active map[string]bool // sessionID -> running
}

// NewChannelRunner builds a runner over the registry + session manager.
func NewChannelRunner(reg *AgentRegistry, sessions *SessionManager) *ChannelRunner {
	return &ChannelRunner{
		Registry: reg,
		Sessions: sessions,
		Lookup:   func(string) channel.Channel { return nil },
		active:   make(map[string]bool),
	}
}

// WithLookup sets the channel lookup used to send replies back.
func (r *ChannelRunner) WithLookup(lookup func(channelID string) channel.Channel) *ChannelRunner {
	r.Lookup = lookup
	return r
}

// RunUserTurn starts an agent run on the session (non-blocking) and streams
// the final text reply back to the channel. Idempotent per session: a turn
// already in flight is dropped (the live run folds the input in, mirroring
// Python's inbox-injection for locked sessions).
func (r *ChannelRunner) RunUserTurn(ctx context.Context, agentID, sessionID string, ev channel.ChannelEvent) error {
	if r.Registry == nil || r.Sessions == nil {
		return nil // no registry/sessions wired; nothing to do
	}
	a, err := r.Registry.Get(ctx, agentID)
	if err != nil {
		return err
	}

	r.mu.Lock()
	if r.active[sessionID] {
		r.mu.Unlock()
		return nil // already running; skip (single-run per session MVP)
	}
	r.active[sessionID] = true
	r.mu.Unlock()

	msg := message.NewMsg().
		Role(message.RoleUser).
		Name(ev.ChannelUserID).
		TextContent(ev.Text).
		Build()

	go r.runAndReply(context.Background(), sessionID, a, ev, msg)
	return nil
}

func (r *ChannelRunner) runAndReply(ctx context.Context, sessionID string, a agent.Agent, ev channel.ChannelEvent, msg *message.Msg) {
	defer func() {
		r.mu.Lock()
		delete(r.active, sessionID)
		r.mu.Unlock()
	}()

	stream, err := r.Sessions.Run(ctx, sessionID, a, msg)
	if err != nil {
		return
	}
	reply := collectFinalText(stream)
	if reply == "" {
		return
	}
	if ch := r.Lookup(ev.ChannelID); ch != nil {
		_ = ch.SendText(ctx, ev.ChatID, reply)
	}
}

// collectFinalText drains the event stream and returns the last assistant
// text block content (a bounded wait on the stream; nil/closed stream yields "").
func collectFinalText(stream <-chan event.AgentEvent) string {
	var b strings.Builder
	for ev := range stream {
		if t, ok := ev.(*event.TextBlockDeltaEvent); ok {
			b.WriteString(t.Delta)
		}
	}
	return b.String()
}
