package model

import (
	"encoding/json"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
)

// TokenCountable is implemented by ChatModel providers that expose accurate token counting.
type TokenCountable interface {
	CountTokens(messages []*message.Msg, tools []ToolSpec) (int, error)
}

// CountTokens estimates input tokens for messages and optional tool schemas.
// Default implementation divides UTF-8 byte length by 4 (aligned with PyV2 ChatModelBase.count_tokens).
func CountTokens(m ChatModel, messages []*message.Msg, tools []ToolSpec) (int, error) {
	if tc, ok := m.(TokenCountable); ok {
		return tc.CountTokens(messages, tools)
	}
	return DefaultCountTokens(messages, tools)
}

// DefaultCountTokens estimates tokens without a model-specific tokenizer.
func DefaultCountTokens(messages []*message.Msg, tools []ToolSpec) (int, error) {
	cnt := 0
	var acc []string

	for _, msg := range messages {
		if msg == nil {
			continue
		}
		for _, block := range msg.Content {
			switch b := block.(type) {
			case *message.TextBlock:
				acc = append(acc, b.Text)
			case *message.ThinkingBlock:
				acc = append(acc, b.Thinking)
			case *message.HintBlock:
				acc = append(acc, b.Text)
			case *message.ToolUseBlock:
				if b.RawInput != "" {
					acc = append(acc, b.RawInput)
				} else if len(b.Input) > 0 {
					raw, err := json.Marshal(b.Input)
					if err != nil {
						return 0, err
					}
					acc = append(acc, string(raw))
				}
			case *message.ToolResultBlock:
				for _, sub := range b.Content {
					if tb, ok := sub.(*message.TextBlock); ok {
						acc = append(acc, tb.Text)
					} else if db, ok := sub.(*message.DataBlock); ok {
						cnt += dataBlockTokens(db)
					}
				}
			case *message.DataBlock:
				cnt += dataBlockTokens(b)
			case *message.ImageBlock:
				if b.Base64 != "" {
					cnt += len(b.Base64) / 4
				} else {
					acc = append(acc, b.URL)
				}
			case *message.AudioBlock:
				acc = append(acc, b.URL)
			case *message.VideoBlock:
				acc = append(acc, b.URL)
			}
		}
	}

	if len(tools) > 0 {
		raw, err := json.Marshal(tools)
		if err != nil {
			return 0, err
		}
		acc = append(acc, string(raw))
	}

	if len(acc) > 0 {
		text := strings.Join(acc, "")
		cnt += int(float64(len([]byte(text)))/4 + 0.5)
	}
	return cnt, nil
}

func dataBlockTokens(b *message.DataBlock) int {
	if b == nil || b.Source == nil {
		return 0
	}
	if b.Source.Type == message.SourceTypeBase64 {
		return len(b.Source.Data) / 4
	}
	return (len(b.Source.URL) + 3) / 4
}
