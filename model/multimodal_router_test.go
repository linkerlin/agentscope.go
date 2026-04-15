package model

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

type mockModel struct {
	name string
}

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.name).Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	ch := make(chan *StreamChunk, 2)
	ch <- &StreamChunk{Delta: m.name}
	ch <- &StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockModel) ModelName() string { return m.name }

func TestMultimodalRouter_TextOnlyUsesDefault(t *testing.T) {
	textModel := &mockModel{name: "text-model"}
	visionModel := &mockModel{name: "vision-model"}
	router := NewMultimodalRouter(textModel, visionModel)

	resp, err := router.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "text-model" {
		t.Fatalf("expected text-model, got %s", resp.GetTextContent())
	}
}

func TestMultimodalRouter_ImageUsesVision(t *testing.T) {
	textModel := &mockModel{name: "text-model"}
	visionModel := &mockModel{name: "vision-model"}
	router := NewMultimodalRouter(textModel, visionModel)

	resp, err := router.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).
			TextContent("describe this").
			Content(message.NewImageBlock("http://example.com/img.png", "", "image/png")).
			Build(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "vision-model" {
		t.Fatalf("expected vision-model, got %s", resp.GetTextContent())
	}
}

func TestMultimodalRouter_VideoUsesVision(t *testing.T) {
	textModel := &mockModel{name: "text-model"}
	visionModel := &mockModel{name: "vision-model"}
	router := NewMultimodalRouter(textModel, visionModel)

	resp, err := router.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).
			TextContent("summarize").
			Content(message.NewVideoBlock("http://example.com/vid.mp4")).
			Build(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "vision-model" {
		t.Fatalf("expected vision-model, got %s", resp.GetTextContent())
	}
}

func TestMultimodalRouter_ChatStream(t *testing.T) {
	textModel := &mockModel{name: "text-model"}
	visionModel := &mockModel{name: "vision-model"}
	router := NewMultimodalRouter(textModel, visionModel)

	ch, err := router.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).
			Content(message.NewAudioBlock("http://example.com/audio.mp3", "", "audio/mp3")).
			Build(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var deltas []string
	for chunk := range ch {
		if chunk != nil && !chunk.Done {
			deltas = append(deltas, chunk.Delta)
		}
	}
	if len(deltas) != 1 || deltas[0] != "vision-model" {
		t.Fatalf("expected vision-model stream, got %v", deltas)
	}
}
