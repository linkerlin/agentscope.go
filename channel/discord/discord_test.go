package discord

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/linkerlin/agentscope.go/channel"
)

// TestMessageNormalization verifies that handleMessageCreate normalises a
// Discord message into a ChannelEvent with the right fields, and ignores
// self-messages and empty messages.
func TestMessageNormalization(t *testing.T) {
	var mu sync.Mutex
	var got []channel.ChannelEvent
	c := New("discord-1", "Bot test-token")
	c.botID = "bot-123" // simulate identified bot

	// emit collector
	c.emit = func(ev channel.ChannelEvent) error {
		mu.Lock()
		got = append(got, ev)
		mu.Unlock()
		return nil
	}

	// normal user message with attachment
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "m1",
			ChannelID: "chan-1",
			Content:   "hello world",
			Author:    &discordgo.User{ID: "user-1", Username: "alice"},
			Attachments: []*discordgo.MessageAttachment{
				{URL: "https://cdn.example.com/a.png"},
			},
		},
	}
	c.handleMessageCreate(nil, msg)

	// self-message → ignored
	c.handleMessageCreate(nil, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID: "m2", ChannelID: "chan-1", Content: "self reply",
			Author: &discordgo.User{ID: "bot-123", Username: "mybot"},
		},
	})

	// nil message → ignored (no panic)
	c.handleMessageCreate(nil, nil)

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	ev := got[0]
	if ev.ChannelID != "discord-1" || ev.ChatID != "chan-1" || ev.Text != "hello world" {
		t.Fatalf("event wrong: %+v", ev)
	}
	if ev.ChannelUserID != "user-1" || ev.ChannelUserName != "alice" {
		t.Fatalf("user fields wrong: %+v", ev)
	}
	if len(ev.MediaURLs) != 1 || ev.MediaURLs[0] != "https://cdn.example.com/a.png" {
		t.Fatalf("media urls wrong: %+v", ev.MediaURLs)
	}
}

// TestChannelImplementsInterface is a compile-time check.
func TestChannelImplementsInterface(t *testing.T) {
	var _ channel.Channel = (*Channel)(nil)
}

// TestNewTrimsBotPrefix verifies the token normalization ("Bot " not doubled).
func TestNewTrimsBotPrefix(t *testing.T) {
	c := New("id", "Bot raw-token")
	if c.token != "raw-token" {
		t.Fatalf("token not normalized: %q", c.token)
	}
	// with the prefix already stripped by New, Start re-adds it
	c2 := New("id2", "Bot abc")
	_ = c2
}

// TestSendTextNotConnected verifies SendText errors before Start.
func TestSendTextNotConnected(t *testing.T) {
	c := New("id", "Bot x")
	if err := c.SendText(context.Background(), "chan-1", "hi"); err == nil {
		t.Fatal("expected error when session not connected")
	}
}

// TestCloseIdempotent verifies Close is safe to call multiple times.
func TestCloseIdempotent(t *testing.T) {
	c := New("id", "Bot x")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	_ = time.Now
}
