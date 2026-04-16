package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/pipeline"
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

	// Step 1: generate ideas
	brainstorm, _ := react.Builder().
		Name("Brainstorm").
		SysPrompt("You are a creative assistant. Generate 3 short ideas.").
		Model(chatModel).
		Build()

	// Step 2: select the best idea
	selector, _ := react.Builder().
		Name("Selector").
		SysPrompt("Pick the best idea and reply with a single sentence starting with 'Best idea:'.").
		Model(chatModel).
		Build()

	// Step 3: expand into a paragraph
	expander, _ := react.Builder().
		Name("Expander").
		SysPrompt("Expand the given idea into a short paragraph.").
		Model(chatModel).
		Build()

	// Use Parallel to get two different perspectives, then pipeline the rest
	par := workflow.NewParallel("Perspectives", nil,
		brainstorm,
		selector,
	)

	pipe := pipeline.New("CreativePipe", par, expander)

	resp, err := pipe.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("sustainable city transport").
		Build())
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Final Output ===")
	fmt.Println(resp.GetTextContent())

	// Demonstrate Condition: route to detail agent only if output is long
	detailAgent, _ := react.Builder().
		Name("DetailAgent").
		SysPrompt("Add actionable bullet points.").
		Model(chatModel).
		Build()

	cond := workflow.NewCondition("DetailCheck",
		func(m *message.Msg) bool { return len(m.GetTextContent()) > 50 },
		detailAgent,
		&mockAgent{name: "Skipper", resp: "(too short for details)"},
	)

	detailResp, _ := cond.Call(context.Background(), resp)
	fmt.Println("\n=== Detail Check ===")
	fmt.Println(detailResp.GetTextContent())
}

type mockAgent struct {
	name string
	resp string
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.resp).Build(), nil
}
func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("not implemented")
}
