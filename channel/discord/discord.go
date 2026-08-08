// Package discord implements a Channel adapter for the Discord platform using
// the discordgo SDK (WebSocket Gateway + REST). Inbound messages arrive via
// the gateway handler; replies are sent via the REST API.
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/linkerlin/agentscope.go/channel"
)

// Channel is the Discord platform adapter. It keeps a long-lived WebSocket
// gateway connection (Start), normalises incoming MessageCreate events into
// channel.ChannelEvent, and sends replies back via the REST API.
type Channel struct {
	id      string
	token   string
	botID   string // the bot's own user id, to ignore its own messages
	emit    func(channel.ChannelEvent) error
	session *discordgo.Session
	Log     *slog.Logger
}

// New creates a Discord channel. token is the bot token; a leading "Bot "
// prefix is normalised away (Start re-adds it for discordgo).
func New(id, token string) *Channel {
	return &Channel{id: id, token: strings.TrimPrefix(token, "Bot "), Log: slog.Default()}
}

// ID returns the channel instance identifier.
func (c *Channel) ID() string { return c.id }

// Start connects to the Discord gateway and begins listening. emit is invoked
// for each inbound message (never concurrently with itself). Blocks until ctx
// is cancelled, then disconnects.
func (c *Channel) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
	sess, err := discordgo.New("Bot " + strings.TrimPrefix(c.token, "Bot "))
	if err != nil {
		return fmt.Errorf("discord: new session: %w", err)
	}
	c.session = sess
	c.emit = emit

	sess.AddHandler(c.handleMessageCreate)
	// identify the bot's own user to filter self-messages
	u, err := sess.User("@me")
	if err == nil {
		c.botID = u.ID
	}

	if err := sess.Open(); err != nil {
		return fmt.Errorf("discord: open gateway: %w", err)
	}
	c.Log.Info("discord channel connected", "channel_id", c.id, "bot", c.botID)

	<-ctx.Done()
	_ = sess.Close()
	return nil
}

// handleMessageCreate normalises an inbound Discord message into a
// ChannelEvent and emits it. Self-messages and empty messages are ignored.
func (c *Channel) handleMessageCreate(_ *discordgo.Session, m *discordgo.MessageCreate) {
	if c.emit == nil || m == nil || m.Message == nil {
		return
	}
	if c.botID != "" && m.Author != nil && m.Author.ID == c.botID {
		return // ignore the bot's own messages
	}
	userName := ""
	if m.Author != nil {
		userName = m.Author.Username
	}
	ev := channel.ChannelEvent{
		ChannelID:        c.id,
		ChannelUserID:    m.Author.ID,
		ChannelUserName:  userName,
		ChatID:           m.ChannelID, // Discord channels are the chat scope
		ChatName:         m.ChannelID,
		ChannelMessageID: m.ID,
		Text:             m.Content,
		ReceivedAt:       time.Now(),
	}
	if len(m.Attachments) > 0 {
		for _, a := range m.Attachments {
			if a.URL != "" {
				ev.MediaURLs = append(ev.MediaURLs, a.URL)
			}
		}
	}
	_ = c.emit(ev)
}

// SendText sends a text reply to a Discord channel.
func (c *Channel) SendText(ctx context.Context, chatID, text string) error {
	if c.session == nil {
		return fmt.Errorf("discord: session not connected")
	}
	if _, err := c.session.ChannelMessageSend(chatID, text); err != nil {
		return fmt.Errorf("discord: send: %w", err)
	}
	return nil
}

// Close disconnects the gateway if connected (idempotent).
func (c *Channel) Close() error {
	if c.session != nil {
		_ = c.session.Close()
		c.session = nil
	}
	return nil
}

// compile-time assertion that Channel implements channel.Channel.
var _ channel.Channel = (*Channel)(nil)
