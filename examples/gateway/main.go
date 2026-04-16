package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY")
		return
	}

	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		panic(err)
	}

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
	fmt.Println("  POST /chat       -> JSON response")
	fmt.Println("  POST /chat/stream -> SSE stream")
	if err := http.ListenAndServe(":8080", srv); err != nil {
		panic(err)
	}
}
