// Command channel_discord demonstrates the Discord Channel adapter: a bot
// that receives messages, routes them to an agent, and replies in the same
// channel. Requires a real bot token.
//
// Run:
//
//	DISCORD_TOKEN=your-bot-token go run ./examples/channel_discord/
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/channel/discord"
)

// echoRunner replies "echo: <text>" to the originating chat (stand-in for the
// gateway's ChannelRunner, which runs a real agent).
type echoRunner struct{}

func (echoRunner) RunUserTurn(ctx context.Context, agentID, sessionID string, ev channel.ChannelEvent) error {
	// in production this is gateway.NewChannelRunner(registry, sessions)
	ch := registry.Get(ev.ChannelID)
	if ch == nil {
		return fmt.Errorf("no channel %q", ev.ChannelID)
	}
	return ch.SendText(ctx, ev.ChatID, "echo: "+ev.Text)
}

var registry = channel.NewRegistry()

func main() {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		fmt.Println("DISCORD_TOKEN 未设置。示例：DISCORD_TOKEN=xxx go run ./examples/channel_discord/")
		os.Exit(1)
	}

	// 1. Discord channel + routing (all chats → agent-1).
	dc := discord.New("discord-1", token)
	dc.Log = slog.Default()
	registry.Register(dc)

	router := channel.NewChatRouter(channel.RouteTable{
		ChannelID: "discord-1",
		Bindings:  []channel.Binding{{AgentID: "agent-1", SessionPrefix: "dc-"}},
	})
	gateway := channel.NewGateway(router, echoRunner{})
	dispatcher := channel.NewDispatcher(gateway, registry)

	// 2. Start listening (blocks until Ctrl+C).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Println("Discord bot 已启动（Ctrl+C 退出）…")
	if err := dispatcher.StartAll(ctx); err != nil {
		panic(err)
	}
	select {}
}
