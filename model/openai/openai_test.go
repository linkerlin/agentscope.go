package openai

import (
	"testing"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/message"
)

func TestMsgToOpenAI_TextOnly(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if out[0].Role != "user" || out[0].Content != "hello" {
		t.Fatalf("unexpected message: %+v", out[0])
	}
}

func TestMsgToOpenAI_ImageBlock_MultiContent(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		TextContent("describe").
		Content(message.NewImageBlock("http://example.com/img.png", "", "image/png")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent for image message")
	}
	foundText := false
	foundImage := false
	for _, part := range out[0].MultiContent {
		switch part.Type {
		case goopenai.ChatMessagePartTypeText:
			foundText = true
			if part.Text != "describe" {
				t.Fatalf("expected text 'describe', got %s", part.Text)
			}
		case goopenai.ChatMessagePartTypeImageURL:
			foundImage = true
			if part.ImageURL == nil || part.ImageURL.URL != "http://example.com/img.png" {
				t.Fatalf("unexpected image url: %+v", part.ImageURL)
			}
		}
	}
	if !foundText || !foundImage {
		t.Fatalf("expected both text and image parts, got text=%v image=%v", foundText, foundImage)
	}
}

func TestMsgToOpenAI_ImageBlock_Base64(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewImageBlock("", "iVBORw0KGgo=", "image/png")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent")
	}
	part := out[0].MultiContent[0]
	if part.Type != goopenai.ChatMessagePartTypeImageURL || part.ImageURL == nil {
		t.Fatal("expected image_url part")
	}
	if part.ImageURL.URL != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("unexpected data url: %s", part.ImageURL.URL)
	}
}

func TestMsgToOpenAI_VideoBlock_FallbackToText(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewVideoBlock("http://example.com/vid.mp4")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent")
	}
	part := out[0].MultiContent[0]
	if part.Type != goopenai.ChatMessagePartTypeText {
		t.Fatalf("expected text fallback, got %v", part.Type)
	}
	if part.Text != "[Video: http://example.com/vid.mp4]" {
		t.Fatalf("unexpected fallback text: %s", part.Text)
	}
}

func TestMsgToOpenAI_AudioBlock_FallbackToText(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewAudioBlock("http://example.com/audio.mp3", "", "audio/mp3")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent")
	}
	part := out[0].MultiContent[0]
	if part.Type != goopenai.ChatMessagePartTypeText {
		t.Fatalf("expected text fallback, got %v", part.Type)
	}
	if part.Text != "[Audio: http://example.com/audio.mp3]" {
		t.Fatalf("unexpected fallback text: %s", part.Text)
	}
}

func TestMsgToOpenAI_ToolResults(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock("ok")}, false),
		message.NewToolResultBlock("call_2", []message.ContentBlock{message.NewTextBlock("done")}, false),
	).Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 2 {
		t.Fatalf("expected 2 tool messages, got %d", len(out))
	}
	if out[0].Role != "tool" || out[0].ToolCallID != "call_1" || out[0].Content != "ok" {
		t.Fatalf("unexpected first tool msg: %+v", out[0])
	}
	if out[1].Role != "tool" || out[1].ToolCallID != "call_2" || out[1].Content != "done" {
		t.Fatalf("unexpected second tool msg: %+v", out[1])
	}
}
