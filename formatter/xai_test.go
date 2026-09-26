package formatter

import (
	"testing"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
)

// TestXAIFormatter_FiltersUnsupportedImages ensures only JPEG/PNG images reach
// the xAI API; other formats become a text placeholder instead of a 400.
func TestXAIFormatter_FiltersUnsupportedImages(t *testing.T) {
	f := NewXAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewTextBlock("see:"),
		message.NewImageBlock("", "aGVsbG8=", "image/png"),
		message.NewImageBlock("", "aGVsbG8=", "image/webp"),
		message.NewImageBlock("", "aGVsbG8=", "image/gif"),
	).Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	parts := out[0].MultiContent
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %+v", parts)
	}
	if parts[0].Type != goopenai.ChatMessagePartTypeText {
		t.Fatalf("expected text part first, got %+v", parts[0])
	}
	if parts[1].Type != goopenai.ChatMessagePartTypeImageURL || parts[1].ImageURL == nil {
		t.Fatalf("expected PNG image preserved, got %+v", parts[1])
	}
	for _, idx := range []int{2, 3} {
		if parts[idx].Type != goopenai.ChatMessagePartTypeText {
			t.Fatalf("expected placeholder text at %d, got %+v", idx, parts[idx])
		}
	}
}

// TestXAIFormatter_PassthroughRemoteURLs ensures remote image URLs (whose
// type cannot be known without fetching) are preserved as-is.
func TestXAIFormatter_PassthroughRemoteURLs(t *testing.T) {
	f := NewXAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewImageBlock("https://example.com/a.webp", "", ""),
	).Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out[0].MultiContent) != 1 || out[0].MultiContent[0].Type != goopenai.ChatMessagePartTypeImageURL {
		t.Fatalf("expected remote image preserved, got %+v", out[0].MultiContent)
	}
}

// TestXAIFormatter_TextOnlyUnchanged ensures text-only messages behave exactly
// like the OpenAI formatter.
func TestXAIFormatter_TextOnlyUnchanged(t *testing.T) {
	f := NewXAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || out[0].Content != "hello" || len(out[0].MultiContent) != 0 {
		t.Fatalf("unexpected text-only message: %+v", out)
	}
}
