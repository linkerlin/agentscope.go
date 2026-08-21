package main

import (
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/internal/llmenv"
)

func main() {
	chatModel := llmenv.MustChatModel()

	agent, err := react.Builder().
		Name("GatewayAgent").
		SysPrompt("You are a helpful assistant accessible via HTTP.").
		Model(chatModel).
		Build()
	if err != nil {
		panic(err)
	}

	srv := gateway.NewServer(agent)
	fmt.Println("Gateway listening on http://localhost:8080")
	fmt.Println("  POST /chat        -> JSON response")
	fmt.Println("  POST /chat/stream -> SSE stream")
	fmt.Println("  GET  /chat/ws     -> WebSocket stream")
	if err := http.ListenAndServe(":8080", srv); err != nil {
		panic(err)
	}
}
