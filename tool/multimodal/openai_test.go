package multimodal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type mockChatModel struct {
	modelName string
	respText  string
}

func (m *mockChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.respText).Build(), nil
}

func (m *mockChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockChatModel) ModelName() string { return m.modelName }

func TestOpenAIMultiModalTool_TextToImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/images/generations" {
			_ = r.ParseForm()
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 123,
				"data": []map[string]any{
					{"url": "https://example.com/img.png"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewOpenAIMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	toolFn := mmt.TextToImageTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(resp.Content))
	}
	data, ok := resp.Content[0].(*message.DataBlock)
	if !ok {
		t.Fatalf("expected DataBlock, got %T", resp.Content[0])
	}
	if data.BlockType() != message.TypeImage {
		t.Fatalf("expected image block")
	}
	if data.Source == nil || data.Source.URL != "https://example.com/img.png" {
		t.Fatalf("unexpected source: %+v", data.Source)
	}
}

func TestOpenAIMultiModalTool_TextToImage_Base64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/images/generations" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"b64_json": "ZmFrZQ=="},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewOpenAIMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	toolFn := mmt.TextToImageTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"prompt": "a dog",
		"response_format": "b64_json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resp.Content[0].(*message.DataBlock)
	if !ok {
		t.Fatalf("expected DataBlock, got %T", resp.Content[0])
	}
	if data.Source == nil || data.Source.Data != "ZmFrZQ==" {
		t.Fatalf("unexpected base64 data")
	}
}

func TestOpenAIMultiModalTool_ImageToText(t *testing.T) {
	mock := &mockChatModel{modelName: "gpt-4o", respText: "A fluffy cat."}
	mmt := NewOpenAIMultiModalToolWithClient("sk-test", "", nil, mock)
	toolFn := mmt.ImageToTextTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"image_urls": "https://example.com/cat.png",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resp.GetTextContent()
	if text != "A fluffy cat." {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestOpenAIMultiModalTool_ImageToText_MultipleURLs(t *testing.T) {
	mock := &mockChatModel{modelName: "gpt-4o", respText: "Two cats."}
	mmt := NewOpenAIMultiModalToolWithClient("sk-test", "", nil, mock)
	toolFn := mmt.ImageToTextTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"image_urls": "https://example.com/a.png, https://example.com/b.png",
		"prompt":     "What do you see?",
		"max_tokens": 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.GetTextContent(), "Two cats") {
		t.Fatalf("unexpected text: %s", resp.GetTextContent())
	}
}

func TestOpenAIMultiModalTool_TextToImage_MissingPrompt(t *testing.T) {
	mmt := NewOpenAIMultiModalToolWithClient("sk-test", "", nil, nil)
	toolFn := mmt.TextToImageTool()
	resp, err := toolFn.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error text")
	}
}
