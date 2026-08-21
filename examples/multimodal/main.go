package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/internal/llmenv"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool/multimodal"
	"github.com/linkerlin/agentscope.go/toolkit"
)

func main() {
	// This example demonstrates how to wire OpenAI multimodal tools into a
	// ReActAgent using toolkit.Toolkit. It requires a real OpenAI API key.
	cfg := llmenv.Load()
	if cfg.APIKey == "" {
		fmt.Println("This example requires an OpenAI API key.")
		fmt.Println("Set the OPENAI_API_KEY environment variable (or a .env file) and run again.")
		return
	}

	// 1. Create an OpenAI-backed chat model for the agent.
	chatModel := llmenv.MustChatModel()

	// 2. Create the multimodal tool wrapper.
	//    This provides openai_text_to_image and openai_image_to_text.
	mmTool, err := multimodal.NewOpenAIMultiModalTool(cfg.APIKey)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Register the multimodal tools into a toolkit.
	//    Toolkit aggregates a registry, groups, and an executor.
	tk := toolkit.NewToolkit()
	if err := tk.Register(mmTool.TextToImageTool()); err != nil {
		log.Fatal(err)
	}
	if err := tk.Register(mmTool.ImageToTextTool()); err != nil {
		log.Fatal(err)
	}

	// 4. Build the ReActAgent with the toolkit attached.
	//    The agent will expose the registered tools to the model via ToolSpec.
	agent, err := react.Builder().
		Name("MultimodalAgent").
		SysPrompt("You are a creative assistant that can generate and analyze images.").
		Model(chatModel).
		Toolkit(tk).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 5. Send a request that may trigger openai_text_to_image or openai_image_to_text.
	response, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Generate an image of a futuristic city and describe it.").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Agent response: %s\n", response.GetTextContent())
}
