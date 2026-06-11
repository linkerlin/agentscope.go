//go:build ignore

// generate_go.go generates JSON fixtures from Go message/event types
// for cross-language compatibility testing with Python agentscope v2.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

func main() {
	msg := message.NewMsg().
		Role(message.RoleAssistant).
		Content(
			message.NewTextBlock("Hello from Go"),
			message.NewImageBlock("", "iVBORw0KGgo=", "image/png"),
			message.NewAudioBlock("http://example.com/audio.mp3", "", "audio/mp3"),
			message.NewVideoBlock("http://example.com/video.mp4"),
			message.NewDataBlock(message.TypeImage, &message.Source{Type: message.SourceTypeURL, URL: "http://example.com/data.png", MediaType: "image/png"}),
			message.NewThinkingBlock("Thinking...", "sig123"),
			message.NewHintBlock("hint text", "suggestion"),
			message.NewToolUseBlock("tu1", "calc", map[string]any{"x": 1}),
			message.NewToolResultBlock("tu1", []message.ContentBlock{message.NewTextBlock("result")}, false),
		).
		Metadata("key", "value").
		Build()
	msg.ID = "msg-go-001"
	msg.CreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("tests/cross_lang/fixtures/go_msg.json", data, 0644); err != nil {
		panic(err)
	}

	// Also generate a simple event-like JSON with nested source structure
	raw := map[string]any{
		"id":          "evt-go-001",
		"type":        "text_block_delta",
		"delta":       "hello",
		"block_index": 0,
		"reply_id":    "reply-001",
		"timestamp":   "2024-01-01T00:00:00Z",
	}
	data2, _ := json.MarshalIndent(raw, "", "  ")
	_ = os.WriteFile("tests/cross_lang/fixtures/go_event.json", data2, 0644)

	fmt.Println("Go fixtures generated successfully")
}
