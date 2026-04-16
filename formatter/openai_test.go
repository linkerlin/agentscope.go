package formatter

import (
	"testing"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// compile-time check that OpenAIFormatter implements Formatter
var _ Formatter = (*OpenAIFormatter)(nil)

func TestOpenAIFormatter_FormatMessages_Interface(t *testing.T) {
	f := NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	result, err := f.FormatMessages([]*message.Msg{msg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	typed, ok := result.([]goopenai.ChatCompletionMessage)
	if !ok {
		t.Fatalf("expected []ChatCompletionMessage, got %T", result)
	}
	if len(typed) != 1 || typed[0].Content != "hi" {
		t.Fatalf("unexpected result: %+v", typed)
	}
}

func TestOpenAIFormatter_FormatMessagesTyped(t *testing.T) {
	f := NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	result := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(result) != 1 || result[0].Content != "hello" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIFormatter_FormatTools_Interface(t *testing.T) {
	f := NewOpenAIFormatter()
	result, err := f.FormatTools([]model.ToolSpec{{Name: "calc", Description: "calc", Parameters: map[string]any{}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	typed, ok := result.([]goopenai.Tool)
	if !ok {
		t.Fatalf("expected []Tool, got %T", result)
	}
	if len(typed) != 1 || typed[0].Function.Name != "calc" {
		t.Fatalf("unexpected result: %+v", typed)
	}
}

func TestOpenAIFormatter_FormatToolsTyped(t *testing.T) {
	f := NewOpenAIFormatter()
	result := f.FormatToolsTyped([]model.ToolSpec{{Name: "calc", Description: "calc", Parameters: map[string]any{}}})
	if len(result) != 1 || result[0].Function.Name != "calc" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIFormatter_FormatToolChoice_Interface(t *testing.T) {
	f := NewOpenAIFormatter()
	result, err := f.FormatToolChoice(&model.ToolChoice{Mode: "auto"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "auto" {
		t.Fatalf("expected auto, got %v", result)
	}
}

func TestOpenAIFormatter_ParseResponse(t *testing.T) {
	f := NewOpenAIFormatter()
	resp := goopenai.ChatCompletionChoice{
		Message: goopenai.ChatCompletionMessage{
			Role:    "assistant",
			Content: "ok",
		},
	}
	msg, err := f.ParseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.GetTextContent() != "ok" {
		t.Fatalf("expected ok, got %s", msg.GetTextContent())
	}
}
