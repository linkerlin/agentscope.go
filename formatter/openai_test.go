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


func TestOpenAIFormatter_FormatToolChoice_Nil(t *testing.T) {
	f := NewOpenAIFormatter()
	result, err := f.FormatToolChoice(nil)
	if err != nil || result != nil {
		t.Fatalf("expected nil, got %v %v", result, err)
	}
}

func TestOpenAIFormatter_FormatToolChoice_Function(t *testing.T) {
	f := NewOpenAIFormatter()
	result, err := f.FormatToolChoice(&model.ToolChoice{Function: "get_weather"})
	if err != nil {
		t.Fatal(err)
	}
	tc, ok := result.(goopenai.ToolChoice)
	if !ok || tc.Type != goopenai.ToolTypeFunction || tc.Function.Name != "get_weather" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIFormatter_ParseChoice_WithToolCalls(t *testing.T) {
	f := NewOpenAIFormatter()
	choice := goopenai.ChatCompletionChoice{
		Message: goopenai.ChatCompletionMessage{
			Role: "assistant",
			ToolCalls: []goopenai.ToolCall{
				{ID: "call_1", Type: goopenai.ToolTypeFunction, Function: goopenai.FunctionCall{Name: "echo", Arguments: `{"x":1}`}},
			},
		},
	}
	msg := f.ParseChoice(choice)
	calls := msg.GetToolUseCalls()
	if len(calls) != 1 || calls[0].Name != "echo" {
		t.Fatalf("unexpected tool calls: %+v", calls)
	}
}

func TestOpenAIFormatter_ParseResponse_WrongType(t *testing.T) {
	f := NewOpenAIFormatter()
	_, err := f.ParseResponse("not a choice")
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestOpenAIFormatter_FormatMessages_WithMedia(t *testing.T) {
	f := NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		TextContent("describe").
		Content(message.NewImageBlock("", "base64data", "image/png")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent for media")
	}
	if out[0].MultiContent[1].Type != goopenai.ChatMessagePartTypeImageURL {
		t.Fatalf("expected image URL part, got %+v", out[0].MultiContent[1])
	}
}

func TestOpenAIFormatter_FormatMessages_AudioAndVideo(t *testing.T) {
	f := NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewAudioBlock("", "audiodata", "audio/wav")).
		Content(message.NewVideoBlock("http://example.com/vid.mp4")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) != 2 {
		t.Fatalf("expected 2 parts, got %+v", out)
	}
}

func TestOpenAIFormatter_FormatMessages_DataBlock(t *testing.T) {
	f := NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewDataBlock(message.TypeImage, &message.Source{Data: "imgdata", MediaType: "image/png"})).
		Content(message.NewDataBlock(message.TypeAudio, &message.Source{Data: "auddata", MediaType: "audio/wav"})).
		Content(message.NewDataBlock(message.TypeVideo, &message.Source{URL: "http://vid"})).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) != 3 {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestOpenAIFormatter_FormatMessages_ToolResults(t *testing.T) {
	f := NewOpenAIFormatter()
	tr := message.NewToolResultBlock("call_1", []message.ContentBlock{
		message.NewTextBlock("result text"),
	}, false)
	msg := message.NewMsg().Role(message.RoleTool).Content(tr).Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || out[0].Role != goopenai.ChatMessageRoleTool || out[0].Content != "result text" {
		t.Fatalf("unexpected tool result message: %+v", out)
	}
}

func TestOpenAIFormatter_FormatMessages_ThinkingBlockSkipped(t *testing.T) {
	f := NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewThinkingBlock("thought", "sig")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) != 0 {
		t.Fatalf("expected thinking block to be skipped, got %+v", out)
	}
}
