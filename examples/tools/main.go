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

	type calcInput struct {
		Operation string  `json:"operation" desc:"The arithmetic operation to perform"`
		A         float64 `json:"a" desc:"First operand"`
		B         float64 `json:"b" desc:"Second operand"`
	}

	calculatorTool := tool.NewFunctionToolAuto("calculator", "Perform basic arithmetic operations",
		func(ctx context.Context, input calcInput) (*tool.Response, error) {
			var result any
			switch input.Operation {
			case "add":
				result = input.A + input.B
			case "subtract":
				result = input.A - input.B
			case "multiply":
				result = input.A * input.B
			case "divide":
				if input.B == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				result = input.A / input.B
			default:
				return nil, fmt.Errorf("unknown operation: %s", input.Operation)
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
