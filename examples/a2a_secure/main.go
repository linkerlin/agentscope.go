// examples/a2a_secure/main.go
//
// Demo: Secure A2A server with API key auth, rate limiting, and WebSocket.
//
// This demo shows how to create a SecureServer, add an API key, attach a
// RateLimiter, and wrap it in a WebSocket-enabled server. No real credentials
// are needed to compile.
//
// How to run:
//   cd examples/a2a_secure && go run main.go
//   curl -H "X-API-Key: demo-key" http://localhost:8080/.well-known/agent.json

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
)

// stubRunner is a mock agent runner for demo purposes.
type stubRunner struct{}

func (s *stubRunner) Run(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
	return &a2a.Message{Role: "agent", Content: "echo: " + msg.Content}, nil
}

func main() {
	// 1. Define the agent card.
	card := a2a.AgentCard{
		Name:         "secure-demo-agent",
		Description:  "A2A secure server demo",
		URL:          ":8080",
		Version:      "1.0.0",
		Capabilities: []string{"text"},
	}

	// 2. Create a secure server with auth and default rate limiting.
	secure := a2a.NewSecureServer(card, &stubRunner{}, nil)

	// 3. Add an API key for authentication.
	auth := a2a.NewAuthMiddleware()
	auth.AddAPIKey("demo-key", "demo-user")
	secure.WithAuth(auth)

	// 4. Configure a custom rate limiter (10 req/s, burst 20).
	limiter := a2a.NewRateLimiter(10, 20)
	secure.WithRateLimit(limiter)

	// 5. Wrap in a WebSocket-enabled server.
	wsServer := a2a.NewWebSocketEnabledServer(card, &stubRunner{}, nil)
	// Re-apply security settings to the WebSocket-enabled server.
	wsAuth := a2a.NewAuthMiddleware()
	wsAuth.AddAPIKey("demo-key", "demo-user")
	wsServer.WithAuth(wsAuth)
	wsServer.WithRateLimit(limiter)

	// 6. Start the HTTP server.
	fmt.Println("starting secure A2A server on", card.URL)
	fmt.Println("try: curl -H 'X-API-Key: demo-key' http://localhost:8080/.well-known/agent.json")
	server := &http.Server{Addr: card.URL, Handler: wsServer, ReadHeaderTimeout: 5 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		fmt.Println("server error:", err)
	}
}
