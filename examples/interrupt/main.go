package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// slowModel sleeps for a fixed duration before returning so that an
// interrupt triggered from another goroutine has time to fire.
type slowModel struct {
	response string
	delay    time.Duration
}

func (m *slowModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	select {
	case <-time.After(m.delay):
		return message.NewMsg().
			Role(message.RoleAssistant).
			TextContent(m.response).
			Build(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *slowModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 1)
	go func() {
		<-time.After(m.delay)
		ch <- &model.StreamChunk{Delta: m.response, Done: true}
	}()
	return ch, nil
}

func (m *slowModel) ModelName() string { return "slow-model" }

func main() {
	// 1. Create a ReActAgent with a slow model.
	//    The 500 ms delay gives the interrupt goroutine enough time to fire.
	agent, err := react.Builder().
		Name("InterruptibleAgent").
		SysPrompt("You are a helpful assistant.").
		Model(&slowModel{response: "This should not appear", delay: 500 * time.Millisecond}).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Start a goroutine that interrupts the agent after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Println(">>> Interrupting agent...")
		agent.Interrupt()
	}()

	// 3. Call the agent. Because the model is slow, the interrupt flag will be
	//    set while the model is still "thinking". When the model call finishes,
	//    the ReAct loop checks the interrupt and returns a recovery message.
	response, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Tell me a long story.").
		Build())
	if err != nil {
		log.Fatalf("agent call failed: %v", err)
	}

	fmt.Printf("Agent response: %s\n", response.GetTextContent())
}
