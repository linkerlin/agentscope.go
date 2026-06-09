package model

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestDefaultCountTokens_TextOnly(t *testing.T) {
	text := strings.Repeat("a", 40)
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(text).Build(),
	}
	n, err := DefaultCountTokens(msgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 10 {
		t.Fatalf("expected 10 tokens, got %d", n)
	}
}

func TestDefaultCountTokens_WithTools(t *testing.T) {
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}
	tools := []ToolSpec{{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}}}
	n, err := DefaultCountTokens(msgs, tools)
	if err != nil || n < 2 {
		t.Fatalf("expected positive token count, got %d err=%v", n, err)
	}
}

func TestCountTokens_UsesTokenCountable(t *testing.T) {
	m := &fixedTokenModel{count: 42}
	n, err := CountTokens(m, nil, nil)
	if err != nil || n != 42 {
		t.Fatalf("expected 42, got %d err=%v", n, err)
	}
}

type fixedTokenModel struct {
	count int
}

func (m *fixedTokenModel) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	return nil, nil
}
func (m *fixedTokenModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	return nil, nil
}
func (m *fixedTokenModel) ModelName() string { return "fixed" }
func (m *fixedTokenModel) CountTokens(messages []*message.Msg, tools []ToolSpec) (int, error) {
	return m.count, nil
}
