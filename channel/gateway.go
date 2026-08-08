package channel

import (
	"context"
	"fmt"
	"log/slog"
)

// Gateway orchestrates inbound channel events: route to an agent/session, then
// deliver as a user turn. It is the Go counterpart of Python's
// ChannelGateway — deliberately thin, no reply collection (output flows back
// through the dispatcher's channel-bound event stream).
type Gateway struct {
	Router Router
	Runner Runner
	Log    *slog.Logger
}

// NewGateway creates a gateway over the given router + runner.
func NewGateway(r Router, runner Runner) *Gateway {
	return &Gateway{Router: r, Runner: runner, Log: slog.Default()}
}

// HandleEvent processes one inbound event. A nil event or one without a chat
// id is ignored. Errors are logged but never propagated (a bad event must not
// kill the channel listener).
func (g *Gateway) HandleEvent(ctx context.Context, ev ChannelEvent) error {
	if ev.ChannelID == "" || ev.ChatID == "" {
		return nil
	}
	if g.Router == nil || g.Runner == nil {
		return fmt.Errorf("channel: gateway missing router or runner")
	}
	agentID, sessionID, err := g.Router.Resolve(ctx, ev)
	if err != nil {
		g.Log.Error("channel: route event", "error", err, "channel_id", ev.ChannelID, "chat_id", ev.ChatID)
		return err
	}
	if err := g.Runner.RunUserTurn(ctx, agentID, sessionID, ev); err != nil {
		g.Log.Error("channel: run user turn", "error", err,
			"agent_id", agentID, "session_id", sessionID)
		return err
	}
	return nil
}

// Registry holds the set of active channel instances keyed by ID.
type Registry struct {
	channels map[string]Channel
}

// NewRegistry creates an empty channel registry.
func NewRegistry() *Registry {
	return &Registry{channels: make(map[string]Channel)}
}

// Register adds a channel instance (replacing any existing with the same ID).
func (r *Registry) Register(ch Channel) error {
	if ch == nil || ch.ID() == "" {
		return fmt.Errorf("channel: invalid channel")
	}
	r.channels[ch.ID()] = ch
	return nil
}

// Get returns the channel with the given ID, or nil.
func (r *Registry) Get(id string) Channel { return r.channels[id] }

// List returns all registered channels.
func (r *Registry) List() []Channel {
	out := make([]Channel, 0, len(r.channels))
	for _, c := range r.channels {
		out = append(out, c)
	}
	return out
}

// Remove deletes a channel instance without closing it.
func (r *Registry) Remove(id string) { delete(r.channels, id) }

// Dispatcher owns channel lifecycle: it starts every registered channel,
// fans inbound events into the Gateway, and stops everything on Close.
type Dispatcher struct {
	Gateway *Gateway
	Reg     *Registry
	Log     *slog.Logger
}

// NewDispatcher creates a dispatcher over the given gateway and registry.
func NewDispatcher(g *Gateway, reg *Registry) *Dispatcher {
	return &Dispatcher{Gateway: g, Reg: reg, Log: slog.Default()}
}

// StartAll begins listening on every registered channel. Each channel runs in
// its own goroutine; emit fans events to the gateway. Returns once all
// channels have started (start is non-blocking). Use Close to stop.
func (d *Dispatcher) StartAll(ctx context.Context) error {
	if d.Gateway == nil || d.Reg == nil {
		return fmt.Errorf("channel: dispatcher missing gateway or registry")
	}
	for _, ch := range d.Reg.List() {
		c := ch
		go func() {
			emit := func(ev ChannelEvent) error {
				return d.Gateway.HandleEvent(context.Background(), ev)
			}
			if err := c.Start(ctx, emit); err != nil && ctx.Err() == nil {
				d.Log.Error("channel: listener exited", "channel_id", c.ID(), "error", err)
			}
		}()
	}
	return nil
}

// Close stops every registered channel (best-effort).
func (d *Dispatcher) Close() {
	for _, c := range d.Reg.List() {
		_ = c.Close()
	}
}
