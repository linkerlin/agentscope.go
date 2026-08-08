package channel

import (
	"context"
	"fmt"
)

// Binding maps inbound chats to a target agent, mirroring Python's
// ChannelBinding. Match rules:
//   - ChatID exact match (highest priority)
//   - ChatIDPrefix prefix match
//   - Default binding used when nothing else matches ("" = any chat)
type Binding struct {
	// ChatID is the exact chat id this binding matches ("" = not used).
	ChatID string
	// ChatIDPrefix matches any chat id starting with this prefix.
	ChatIDPrefix string
	// AgentID is the target agent for the matched chat.
	AgentID string
	// SessionPrefix prefixes the derived session id (e.g. "discord-").
	SessionPrefix string
}

// RouteTable is an ordered list of bindings for one channel. The first
// matching binding wins.
type RouteTable struct {
	// ChannelID is the channel instance this table applies to ("" = default).
	ChannelID string
	// Bindings in priority order (exact > prefix > default).
	Bindings []Binding
}

// matchBinding returns the binding that matches the event's chat, or nil.
func (t RouteTable) matchBinding(ev ChannelEvent) *Binding {
	var prefixMatch *Binding
	var defaultMatch *Binding
	for i := range t.Bindings {
		b := &t.Bindings[i]
		switch {
		case b.ChatID != "" && b.ChatID == ev.ChatID:
			return b // exact wins immediately
		case b.ChatIDPrefix != "" && len(ev.ChatID) >= len(b.ChatIDPrefix) &&
			ev.ChatID[:len(b.ChatIDPrefix)] == b.ChatIDPrefix:
			if prefixMatch == nil {
				prefixMatch = b
			}
		case b.ChatID == "" && b.ChatIDPrefix == "":
			if defaultMatch == nil {
				defaultMatch = b
			}
		}
	}
	if prefixMatch != nil {
		return prefixMatch
	}
	return defaultMatch
}

// Resolve derives (agentID, sessionID) for an event from the route table.
// sessionID is deterministic: <prefix><chat_id>, grouping all users of one
// chat into one session (aligns with Python's per-chat session derivation).
func (t RouteTable) Resolve(ctx context.Context, ev ChannelEvent) (string, string, error) {
	b := t.matchBinding(ev)
	if b == nil || b.AgentID == "" {
		return "", "", fmt.Errorf("channel: no routing binding for chat %q", ev.ChatID)
	}
	sid := b.SessionPrefix + ev.ChatID
	return b.AgentID, sid, nil
}

// ChatRouter routes events for multiple channels via their route tables.
// Implements Router.
type ChatRouter struct {
	// tables keyed by channel id ("" = default table for any channel).
	tables map[string]RouteTable
}

// NewChatRouter creates a router over the given tables.
func NewChatRouter(tables ...RouteTable) *ChatRouter {
	r := &ChatRouter{tables: make(map[string]RouteTable)}
	for _, t := range tables {
		r.tables[t.ChannelID] = t
	}
	return r
}

// AddTable registers (or replaces) a route table.
func (r *ChatRouter) AddTable(t RouteTable) {
	r.tables[t.ChannelID] = t
}

// Resolve routes an event using the table for its channel (falling back to
// the default table "").
func (r *ChatRouter) Resolve(ctx context.Context, ev ChannelEvent) (string, string, error) {
	if t, ok := r.tables[ev.ChannelID]; ok {
		return t.Resolve(ctx, ev)
	}
	if t, ok := r.tables[""]; ok {
		return t.Resolve(ctx, ev)
	}
	return "", "", fmt.Errorf("channel: no route table for channel %q", ev.ChannelID)
}
