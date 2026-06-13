// mapreduce_summary demonstrates summarizing a long document with MapReduce.
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
	"github.com/linkerlin/agentscope.go/workflow"
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

	mapper, err := react.Builder().
		Name("Summarizer").
		SysPrompt("Summarize the given text chunk in one or two sentences.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	reducer, err := react.Builder().
		Name("Synthesizer").
		SysPrompt("Combine the following partial summaries into a single coherent summary.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	longText := `Go is a statically typed, compiled programming language designed at Google. ` +
		`It is syntactically similar to C, but with memory safety, garbage collection, structural typing, and CSP-style concurrency. ` +
		`The language is often referred to as Golang because of its former domain name, golang.org, but its proper name is Go. ` +
		`Go was designed at Google in 2007 to improve programming productivity in an era of multicore, networked machines and large codebases. ` +
		`The designers wanted to address criticisms of other languages used at Google while keeping their useful characteristics. ` +
		`The language is used by many large companies and open-source projects, including Docker, Kubernetes, and Terraform.`

	split := func(m *message.Msg) []string {
		parts := strings.Split(m.GetTextContent(), ". ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
			if parts[i] != "" && !strings.HasSuffix(parts[i], ".") {
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
		log.Fatal(err)
	}

	fmt.Println("=== Final Summary ===")
	fmt.Println(resp.GetTextContent())
}
