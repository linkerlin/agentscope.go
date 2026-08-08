// Command channel_feishu demonstrates the Feishu (Lark) Channel adapter:
// an event-subscription webhook receives messages, routes them to an agent,
// and replies via the REST API. Requires a Feishu Open Platform app.
//
// Run:
//
//	FEISHU_APP_ID=xxx FEISHU_APP_SECRET=yyy go run ./examples/channel_feishu/
//
// Configure the app's event subscription URL to http://<host>:8090/webhook.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/channel/feishu"
)

// echoRunner replies "echo: <text>" to the originating chat.
type echoRunner struct{}

func (echoRunner) RunUserTurn(ctx context.Context, agentID, sessionID string, ev channel.ChannelEvent) error {
	ch := registry.Get(ev.ChannelID)
	if ch == nil {
		return fmt.Errorf("no channel %q", ev.ChannelID)
	}
	return ch.SendText(ctx, ev.ChatID, "echo: "+ev.Text)
}

var registry = channel.NewRegistry()

func main() {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	if appID == "" || appSecret == "" {
		fmt.Println("FEISHU_APP_ID / FEISHU_APP_SECRET 未设置。")
		os.Exit(1)
	}

	// 1. Feishu channel + routing (all chats → agent-1).
	fc := feishu.New("feishu-1", appID, appSecret)
	fc.Log = slog.Default()
	registry.Register(fc)

	router := channel.NewChatRouter(channel.RouteTable{
		ChannelID: "feishu-1",
		Bindings:  []channel.Binding{{AgentID: "agent-1", SessionPrefix: "fs-"}},
	})
	gateway := channel.NewGateway(router, echoRunner{})
	dispatcher := channel.NewDispatcher(gateway, registry)

	// 2. Start (webhook mode: Start just wires the emit callback).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dispatcher.StartAll(ctx); err != nil {
		panic(err)
	}
	defer dispatcher.Close()

	// 3. Serve the event-subscription webhook.
	mux := http.NewServeMux()
	mux.Handle("/webhook", fc)
	addr := ":8090"
	fmt.Printf("Feishu bot 事件订阅: http://localhost%s/webhook\n", addr)
	fmt.Println("在飞书开放平台配置应用事件订阅 URL 为此地址（im.message.receive_v1）")
	if err := http.ListenAndServe(addr, mux); err != nil {
		panic(err)
	}
}
