// Example: MultimodalRouter — automatic routing between text and vision models.
//
// When a user sends a message containing an image, the router transparently
// switches from the default text model (e.g. deepseek-chat) to a vision-capable
// model (e.g. gpt-4o or qwen-vl-plus). No agent code changes needed.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Set OPENAI_API_KEY to run this example.")
		return
	}

	ctx := context.Background()

	// 1. Build two backend models: a fast text model and a vision model.
	textModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	visionModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Wrap them in a MultimodalRouter.
	router := model.NewMultimodalRouter(textModel, visionModel)
	fmt.Printf("Router will use model: %s (auto-detect media)\n", router.ModelName())

	// 3. Build a ReActAgent using the router as its model.
	agent, err := react.Builder().
		Name("RouterAgent").
		SysPrompt("You are a helpful assistant. Describe any images you see.").
		Model(router).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 4. Text-only message → text model (gpt-4o-mini).
	textMsg := message.NewMsg().
		Role(message.RoleUser).
		TextContent("What is 2+2?").
		Build()

	resp, err := agent.Call(ctx, textMsg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Text-only reply: %s\n", resp.GetTextContent())

	// 5. Message with an image → vision model (gpt-4o).
	//    (Replace with an actual image URL for real use.)
	imageMsg := message.NewMsg().
		Role(message.RoleUser).
		Content(message.NewImageBlock("https://example.com/sample.png", "", "image/png")).
		TextContent("What do you see in this image?").
		Build()

	resp2, err := agent.Call(ctx, imageMsg)
	if err != nil {
		fmt.Printf("Image reply failed (expected with placeholder URL): %v\n", err)
	} else {
		fmt.Printf("Image reply: %s\n", resp2.GetTextContent())
	}

	fmt.Println("\n=== How it works ===")
	fmt.Println("MultimodalRouter.selectModel() checks each message for ImageBlock/AudioBlock/VideoBlock.")
	fmt.Println("If any media block is found → routes to visionModel; otherwise → defaultModel.")
	fmt.Println("No agent or tool changes required — transparent automatic routing.")
}
