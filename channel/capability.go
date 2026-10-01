// channel/capability.go realises the capability model (18.6): channels
// declare what the platform can do, and send policy (long-message split,
// reactions, chat listing) is driven by the declaration instead of
// per-adapter special cases. Declaring is optional — the optional-interface
// pattern used across the repo — and an undeclared channel keeps exactly
// its pre-18.6 behaviour (no splitting, no reactions).
package channel

import (
	"context"
	"strings"
)

// Capability names a platform ability a Channel can declare.
type Capability string

const (
	// CapReaction: the platform supports emoji reactions on messages.
	CapReaction Capability = "reaction"
	// CapListChats: the platform supports listing the chats/groups the bot
	// is a member of.
	CapListChats Capability = "list_chats"
	// CapWebSocket: the adapter holds a long-lived inbound connection
	// (rather than HTTP webhook/pull).
	CapWebSocket Capability = "websocket"
	// CapCardStream: the platform supports streaming card updates (AI
	// cards).
	CapCardStream Capability = "card_stream"
)

// CapabilityProvider is the OPTIONAL capability declaration: channels that
// implement it report their platform abilities. Obtain via CapabilitiesOf.
type CapabilityProvider interface {
	Capabilities() []Capability
}

// TextLimitProvider is the OPTIONAL per-message text limit (runes). Obtain
// via MaxTextLenOf; 0 means "no splitting".
type TextLimitProvider interface {
	MaxTextLen() int
}

// CapabilitiesOf returns the channel's declared capabilities; an
// undeclared channel yields nil (policy then does nothing special).
func CapabilitiesOf(c Channel) []Capability {
	if p, ok := c.(CapabilityProvider); ok {
		return p.Capabilities()
	}
	return nil

}

// HasCapability reports whether the channel declares cap.
func HasCapability(c Channel, cap Capability) bool {
	for _, k := range CapabilitiesOf(c) {
		if k == cap {
			return true
		}
	}
	return false
}

// MaxTextLenOf returns the channel's per-message rune limit, 0 when the
// channel does not declare one.
func MaxTextLenOf(c Channel) int {
	if p, ok := c.(TextLimitProvider); ok {
		return p.MaxTextLen()
	}
	return 0
}

// SplitLongText breaks text into chunks of at most maxLen runes, preferring
// paragraph boundaries (double newline), then single newlines, then a hard
// cut — never mid-rune. maxLen <= 0 returns the text unsplit.
func SplitLongText(text string, maxLen int) []string {
	if maxLen <= 0 || len([]rune(text)) <= maxLen {
		return []string{text}
	}
	runes := []rune(text)
	var out []string
	for len(runes) > maxLen {
		cut := maxLen
		window := string(runes[:maxLen])
		// Prefer the last paragraph break, then the last line break.
		if i := strings.LastIndex(window, "\n\n"); i > maxLen/3 {
			cut = i + 2
		} else if i := strings.LastIndex(window, "\n"); i > maxLen/3 {
			cut = i + 1
		}
		out = append(out, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

// DeliverText is the capability-driven send path: long messages are split
// at the channel's declared limit (if any) and delivered in order. The
// first failure aborts the remainder — callers see which chunk was lost.
// Channels without a declared limit send exactly as before.
func DeliverText(ctx context.Context, ch Channel, chatID, text string) error {
	for _, part := range SplitLongText(text, MaxTextLenOf(ch)) {
		if err := ch.SendText(ctx, chatID, part); err != nil {
			return err
		}
	}
	return nil
}
