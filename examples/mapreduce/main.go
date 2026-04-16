package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/workflow"
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

	mapper, _ := react.Builder().
		Name("Summarizer").
		SysPrompt("Summarize the given paragraph in one sentence.").
		Model(chatModel).
		Build()

	reducer, _ := react.Builder().
		Name("Synthesizer").
		SysPrompt("Combine the following summaries into a coherent single-paragraph summary.").
		Model(chatModel).
		Build()

	longText := "Go is a statically typed, compiled programming language designed at Google. " +
		"It is syntactically similar to C, but with memory safety, garbage collection, structural typing, and CSP-style concurrency. " +
		"The language is often referred to as Golang because of its former domain name, golang.org, but its proper name is Go. " +
		"Go was designed at Google in 2007 to improve programming productivity in an era of multicore, networked machines and large codebases."

	split := func(m *message.Msg) []string {
		parts := strings.Split(m.GetTextContent(), ". ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
			if !strings.HasSuffix(parts[i], ".") {
				parts[i] += "."
			}
		}
		return parts
	}

	mr := workflow.NewMapReduce("DocSummary", split, mapper, reducer, 4)
	resp, err := mr.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent(longText).
		Build())
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Final Summary ===")
	fmt.Println(resp.GetTextContent())
}
