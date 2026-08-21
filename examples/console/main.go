// Command console starts an interactive bubbletea TUI chat with a ReAct agent.
//
// LLM config comes from internal/llmenv (OPENAI_API_KEY / OPENAI_BASE_URL /
// OPENAI_MODEL, with .env fallback). The calculator tool requires human
// confirmation (permission ModeDefault), demonstrating the HITL y/n/a flow.
//
// Run:
//
//	go run ./examples/console
//
// Keys: enter send · alt+enter newline · exit/quit or ctrl+d leave ·
// ctrl+c interrupts a running reply.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/console"
	"github.com/linkerlin/agentscope.go/internal/llmenv"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/tool"
)

type calcInput struct {
	Operation string  `json:"operation" desc:"add | subtract | multiply | divide"`
	A         float64 `json:"a" desc:"first operand"`
	B         float64 `json:"b" desc:"second operand"`
}

func main() {
	chatModel := llmenv.MustChatModel()

	calculator := tool.NewFunctionToolAuto("calculator", "Perform basic arithmetic",
		func(_ context.Context, in calcInput) (*tool.Response, error) {
			var r float64
			switch in.Operation {
			case "add":
				r = in.A + in.B
			case "subtract":
				r = in.A - in.B
			case "multiply":
				r = in.A * in.B
			case "divide":
				if in.B == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				r = in.A / in.B
			default:
				return nil, fmt.Errorf("unknown operation: %s", in.Operation)
			}
			return tool.NewTextResponse(fmt.Sprintf("%g %s %g = %g", in.A, in.Operation, in.B, r)), nil
		},
	)

	// ModeDefault asks for confirmation on every tool call (HITL demo).
	perm := permission.NewEngine(permission.ModeDefault, nil)

	ag, err := react.Builder().
		Name("ConsoleBot").
		SysPrompt("You are a helpful assistant. Use the calculator tool for any arithmetic.").
		Model(chatModel).
		Tools(calculator).
		PermissionEngine(perm).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	if err := console.Launch(context.Background(), ag, console.Options{
		Verbosity: console.Default,
	}); err != nil {
		log.Fatal(err)
	}
}
