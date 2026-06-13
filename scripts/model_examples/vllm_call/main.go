// vllm_call demonstrates a simple call via a self-hosted vLLM server.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/vllm"
)

func main() {
	chatModel, err := vllm.Builder("not-needed").
		BaseURL("http://localhost:8000/v1").
		ModelName("meta-llama/Meta-Llama-3-8B-Instruct").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("VLLMAssistant").
		SysPrompt("You are a helpful assistant powered by a local vLLM model.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("What is machine learning?").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
