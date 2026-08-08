// Command channel_webhook demonstrates the multi-platform Channel subsystem
// end-to-end with a zero-dependency webhook channel: an HTTP POST is
// normalised into a ChannelEvent, routed to an agent, run, and the reply is
// sent back to the chat.
//
// Run:
//
//	go run ./examples/channel_webhook/
//
// then POST to the printed endpoint, e.g.:
//
//	curl -X POST http://localhost:8090/webhook \
//	  -H "Content-Type: application/json" \
//	  -d '{"chat_id":"team-1","user_id":"u1","text":"hello channel"}'
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/linkerlin/agentscope.go/channel"
)

// echoRunner simulates the agent side: it "runs" the event (echoes the text)
// and sends the reply back through the owning channel — the same shape as the
// gateway's ChannelRunner adapter.
type echoRunner struct{}

func (echoRunner) RunUserTurn(ctx context.Context, agentID, sessionID string, ev channel.ChannelEvent) error {
	ch := channelRegistry.Get(ev.ChannelID)
	if ch == nil {
		return fmt.Errorf("no channel %q", ev.ChannelID)
	}
	reply := fmt.Sprintf("[%s -> %s] echo: %s", agentID, sessionID, ev.Text)
	go func() {
		time.Sleep(100 * time.Millisecond) // simulate agent latency
		_ = ch.SendText(ctx, ev.ChatID, reply)
	}()
	return nil
}

var channelRegistry = channel.NewRegistry()

func main() {
	// 1. Build the channel subsystem: webhook channel + router + runner.
	wh := channel.NewWebhookChannel("webhook-1")
	wh.Log = slog.Default()
	channelRegistry.Register(wh)

	router := channel.NewChatRouter(channel.RouteTable{
		ChannelID: "webhook-1",
		Bindings: []channel.Binding{
			{ChatIDPrefix: "dev-", AgentID: "dev-agent", SessionPrefix: "dev-"},
			{ChatIDPrefix: "qa-", AgentID: "qa-agent", SessionPrefix: "qa-"},
			{AgentID: "default-agent", SessionPrefix: "d-"},
		},
	})
	gateway := channel.NewGateway(router, echoRunner{})
	dispatcher := channel.NewDispatcher(gateway, channelRegistry)

	// 2. Start the dispatcher (for webhook, Start is a no-op beyond wiring).
	ctx := context.Background()
	if err := dispatcher.StartAll(ctx); err != nil {
		panic(err)
	}
	defer dispatcher.Close()

	// 3. Serve the webhook endpoint.
	mux := http.NewServeMux()
	mux.Handle("/webhook", wh)

	addr := envOr("PORT", "8090")
	fmt.Printf("Channel webhook demo: http://localhost:%s/webhook\n", addr)
	fmt.Println("Post a message: {\"chat_id\":\"dev-1\",\"user_id\":\"u1\",\"text\":\"hi\"}")
	fmt.Println("Replies are echoed back via SendText (printed below).")
	if err := http.ListenAndServe(":"+addr, mux); err != nil {
		panic(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
