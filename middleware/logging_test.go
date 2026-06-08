package middleware_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
)

func TestLoggingMiddleware_OnReply(t *testing.T) {
	var buf bytes.Buffer
	lm := middleware.NewLoggingMiddleware(&buf)
	lm.Prefix = "demo: "

	chain := middleware.Classify([]middleware.Middleware{lm})
	handler := middleware.ChainReply(chain, stubAgent{name: "agent-a"}, &middleware.ReplyInput{
		Messages: []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()},
	}, func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	})
	if _, err := handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "on_reply start") || !strings.Contains(out, "on_reply end") {
		t.Fatalf("expected reply logs, got %q", out)
	}
}

func TestLoggingMiddleware_OnSystemPrompt(t *testing.T) {
	var buf bytes.Buffer
	lm := middleware.NewLoggingMiddleware(&buf)
	chain := middleware.Classify([]middleware.Middleware{lm})
	got, err := middleware.ApplySystemPrompt(context.Background(), stubAgent{name: "a"}, chain, "sys")
	if err != nil || got != "sys" {
		t.Fatalf("prompt=%q err=%v", got, err)
	}
	if !strings.Contains(buf.String(), "on_system_prompt") {
		t.Fatalf("expected system prompt log, got %q", buf.String())
	}
}
