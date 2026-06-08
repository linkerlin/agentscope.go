package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// demoAgent streams synthetic AG-UI-friendly events when no API key is set.
type demoAgent struct{}

func newDemoAgent() agent.V2Agent {
	return &demoAgent{}
}

func (d *demoAgent) Name() string { return "AG-UI Demo" }

func (d *demoAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("Use ReplyStream for AG-UI demo.").Build(), nil
}

func (d *demoAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("Use /v2/chat/stream?protocol=agui").Build()
	close(ch)
	return ch, nil
}

func (d *demoAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return d.Call(ctx, msg)
}

func (d *demoAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 64)
	go d.stream(ctx, msg, ch)
	return ch, nil
}

func (d *demoAgent) stream(ctx context.Context, msg *message.Msg, ch chan<- event.AgentEvent) {
	defer close(ch)
	send := func(ev event.AgentEvent) bool {
		select {
		case ch <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}
	sleep := func(d time.Duration) bool {
		select {
		case <-time.After(d):
			return true
		case <-ctx.Done():
			return false
		}
	}

	replyID := fmt.Sprintf("demo-%d", time.Now().UnixNano())
	userText := msg.GetTextContent()

	if !send(event.NewReplyStart(replyID, d.Name())) {
		return
	}
	if !send(event.NewModelCallStart(replyID, "demo-model")) {
		return
	}

	if !send(event.NewThinkingBlockStart(replyID, 0)) {
		return
	}
	think := fmt.Sprintf("User asked: %q. Preparing AG-UI event stream.", userText)
	for _, part := range chunkText(think, 12) {
		if !send(event.NewThinkingBlockDelta(replyID, 0, part)) || !sleep(25*time.Millisecond) {
			return
		}
	}
	if !send(event.NewThinkingBlockEnd(replyID, 0)) {
		return
	}
	if !send(event.NewModelCallEnd(replyID, "demo-model", 42, 128)) {
		return
	}

	if strings.Contains(strings.ToLower(userText), "tool") {
		toolID := "tc_demo_1"
		if !send(event.NewToolCallStart(replyID, 1, toolID, "get_weather")) {
			return
		}
		args := `{"city":"Shanghai"}`
		for _, part := range chunkText(args, 4) {
			if !send(event.NewToolCallDelta(replyID, 1, toolID, part)) || !sleep(20*time.Millisecond) {
				return
			}
		}
		if !send(event.NewToolCallEnd(replyID, 1, toolID)) {
			return
		}
		if !send(event.NewToolResultStart(replyID, 2, toolID, "get_weather")) {
			return
		}
		if !send(event.NewToolResultTextDelta(replyID, 2, toolID, "Shanghai: 24°C, ")) {
			return
		}
		if !send(event.NewToolResultTextDelta(replyID, 2, toolID, "partly cloudy")) {
			return
		}
		if !send(event.NewToolResultEnd(replyID, 2, toolID)) {
			return
		}
	}

	if !send(event.NewTextBlockStart(replyID, 3)) {
		return
	}
	reply := fmt.Sprintf("Hello! This is the AG-UI demo agent. You said: %s", userText)
	if strings.Contains(userText, "system-notification") || strings.Contains(userText, "Background task") {
		reply += " (Background offload notification detected in your message.)"
	} else if strings.Contains(strings.ToLower(userText), "tool") {
		reply += " Tool call completed — see the card above."
	} else {
		reply += " Try sending a message containing \"tool\" to see tool-call events."
	}
	for _, part := range chunkText(reply, 8) {
		if !send(event.NewTextBlockDelta(replyID, 3, part)) || !sleep(30*time.Millisecond) {
			return
		}
	}
	if !send(event.NewTextBlockEnd(replyID, 3)) {
		return
	}
	send(event.NewReplyEnd(replyID, d.Name()))
}

func (d *demoAgent) LoadState(state *agent.AgentState) error { return nil }
func (d *demoAgent) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (d *demoAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func chunkText(s string, size int) []string {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	var out []string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}
