// multi_agent_review demonstrates a writer-critic-editor pipeline for content review.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/pipeline"
	"github.com/linkerlin/agentscope.go/reflection"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	model, err := openai.Builder().APIKey(apiKey).ModelName("gpt-4o-mini").Build()
	if err != nil {
		log.Fatal(err)
	}

	writer, err := react.Builder().
		Name("Writer").
		SysPrompt("You are a technical writer. Write a concise, engaging paragraph on the given topic.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	critic, err := react.Builder().
		Name("Critic").
		SysPrompt("You are a strict editor. If the draft is already excellent, reply ONLY with 'PASS'. Otherwise give ONE concrete improvement suggestion.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	editor, err := react.Builder().
		Name("Editor").
		SysPrompt("You are a final editor. Polish the draft based on the critique and produce the final version.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Step 1: self-refining writer
	refiningWriter := reflection.NewSelfReflectingAgent(
		"RefiningWriter",
		writer,
		critic,
		func(_, critique *message.Msg) bool {
			return strings.Contains(critique.GetTextContent(), "PASS")
		},
		3,
	)

	// Step 2: final editor polish
	reviewPipe := pipeline.New("ReviewPipeline", refiningWriter, editor)

	resp, err := reviewPipe.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Explain why Go is a good choice for building agent services.").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Final Reviewed Draft ===")
	fmt.Println(resp.GetTextContent())
}
