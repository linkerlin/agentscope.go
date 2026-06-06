//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/message"
)

func main() {
	data, err := os.ReadFile("tests/cross_lang/fixtures/py_msg.json")
	if err != nil {
		fmt.Printf("FAIL: read file: %v\n", err)
		os.Exit(1)
	}

	var msg message.Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		fmt.Printf("FAIL: unmarshal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("OK: parsed %d block(s)\n", len(msg.Content))
	for i, b := range msg.Content {
		fmt.Printf("  block %d: type=%s\n", i, b.BlockType())
	}

	// Verify specific blocks
	if msg.Name != "PyAgent" {
		fmt.Printf("FAIL: expected name PyAgent, got %s\n", msg.Name)
		os.Exit(1)
	}
	if msg.Content[0].(*message.TextBlock).Text != "Hello from Python" {
		fmt.Printf("FAIL: unexpected text block\n")
		os.Exit(1)
	}
	if msg.Content[1].(*message.ImageBlock).URL != "http://example.com/img.png" {
		fmt.Printf("FAIL: unexpected image block\n")
		os.Exit(1)
	}
	if msg.Content[5].(*message.ToolUseBlock).Name != "calc" {
		fmt.Printf("FAIL: unexpected tool use block\n")
		os.Exit(1)
	}
	if msg.Content[6].(*message.ToolResultBlock).ToolUseID != "tc1" {
		fmt.Printf("FAIL: unexpected tool result block\n")
		os.Exit(1)
	}

	fmt.Println("All validations passed")
}
