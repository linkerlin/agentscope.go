// Package channel provides a multi-platform messaging integration layer,
// mirroring Python agentscope's app/channel (#1997). A channel keeps a
// long-lived connection to an external platform (Discord, Feishu, webhook),
// normalises inbound payloads into ChannelEvent, and sends the agent's
// outbound replies back to the platform.
//
// The package is deliberately decoupled from the gateway: channel defines the
// abstractions (Channel / Router / Runner), and the gateway supplies adapters
// so there is no import cycle.
package channel

import (
	"context"
	"time"
)

// ChannelEvent is a normalised inbound message from an external platform,
// aligned with Python's ChannelEvent (channel_id / user / chat / content).
type ChannelEvent struct {
	// ChannelID identifies the channel instance that received this event.
	ChannelID string
	// ChannelUserID is the platform-side unique user identifier.
	ChannelUserID string
	// ChannelUserName is the platform-side display name (may be empty).
	ChannelUserName string
	// ChatID is the platform-side chat/group identifier. Drives session
	// grouping and routing-rule matching.
	ChatID string
	// ChatName is the human-readable chat title (may be empty).
	ChatName string
	// ChannelMessageID is the platform-side message id (for reply refs).
	ChannelMessageID string
	// Text is the text content of the message ("" for media-only).
	Text string
	// MediaURLs are media attachment URLs (may be empty).
	MediaURLs []string
	// Metadata carries platform-specific info (chat_type, tenant_key, ...).
	Metadata map[string]any
	// ReceivedAt records when the platform delivered the event.
	ReceivedAt time.Time
}

// Channel is the platform adapter contract. Implementations keep a long-lived
// connection, normalise inbound payloads into ChannelEvent and pass them to
// emit, and send outbound replies back to the platform.
type Channel interface {
	// ID returns the channel instance identifier (stable across restarts).
	ID() string
	// Start begins listening; emit is called for each inbound event (it must
	// be safe to call concurrently). Blocks until ctx is cancelled or the
	// connection dies. Exactly one Start should be running at a time.
	Start(ctx context.Context, emit func(ChannelEvent) error) error
	// SendText sends a text reply to a chat on the platform.
	SendText(ctx context.Context, chatID, text string) error
	// Close tears down the connection (idempotent).
	Close() error
}

// Router resolves which (agent, session) an inbound event belongs to. The
// gateway supplies an adapter over its own session/agent registry.
type Router interface {
	// Resolve returns the target agent id and derived session id for an event.
	Resolve(ctx context.Context, ev ChannelEvent) (agentID, sessionID string, err error)
}

// Runner delivers a user turn to an agent/session and is responsible for
// streaming the reply back out (e.g. via a channel-bound event stream). The
// gateway supplies an adapter over its SessionManager.
type Runner interface {
	// RunUserTurn starts a user turn for the event against the resolved
	// agent/session. Implementations may return before the run completes.
	RunUserTurn(ctx context.Context, agentID, sessionID string, ev ChannelEvent) error
}
