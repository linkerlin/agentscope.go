package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/reflection"
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

	writer, _ := react.Builder().
		Name("Writer").
		SysPrompt("You are a concise technical writer. Produce a single-paragraph summary.").
		Model(chatModel).
		Build()

	critic, _ := react.Builder().
		Name("Critic").
		SysPrompt("You are a strict editor. If the summary is already excellent, reply ONLY with 'PASS'. Otherwise, give ONE concrete improvement suggestion.").
		Model(chatModel).
		Build()

	agent := reflection.NewSelfReflectingAgent(
		"RefiningWriter",
		writer,
		critic,
		func(_, critique *message.Msg) bool {
			return strings.Contains(critique.GetTextContent(), "PASS")
		},
		3,
	)

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Explain Go channels to a junior developer in one paragraph.").
		Build())
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Final Draft ===")
	fmt.Println(resp.GetTextContent())
}
