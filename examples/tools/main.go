package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/tool"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	calculatorTool := tool.NewFunctionTool(
		"calculator",
		"Perform basic arithmetic operations",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{
					"type":        "string",
					"enum":        []string{"add", "subtract", "multiply", "divide"},
					"description": "The arithmetic operation to perform",
				},
				"a": map[string]any{"type": "number", "description": "First operand"},
				"b": map[string]any{"type": "number", "description": "Second operand"},
			},
			"required": []string{"operation", "a", "b"},
		},
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			op, _ := input["operation"].(string)
			a, _ := input["a"].(float64)
			b, _ := input["b"].(float64)
			var result any
			switch op {
			case "add":
				result = a + b
			case "subtract":
				result = a - b
			case "multiply":
				result = a * b
			case "divide":
				if b == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				result = a / b
			default:
				return nil, fmt.Errorf("unknown operation: %s", op)
			}
			return tool.NewTextResponse(result), nil
		},
	)

	agent, err := react.Builder().
		Name("Calculator").
		SysPrompt("You are a helpful calculator assistant. Use the calculator tool to perform arithmetic.").
		Model(chatModel).
		Tools(calculatorTool).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	response, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("What is 42 multiplied by 17?").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Calculator Agent: %s\n", response.GetTextContent())
}
