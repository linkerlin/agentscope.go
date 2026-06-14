package mcp

import (
	"context"
	"fmt"
	"testing"
)

// --- Mock MCP client with Prompts + Resources ---

type mockFullClient struct {
	mockClient
	prompts    []PromptInfo
	promptRes  *PromptResult
	resources  []ResourceInfo
	resContent *ResourceContent
}

func (m *mockFullClient) ListPrompts(ctx context.Context) ([]PromptInfo, error) {
	return m.prompts, nil
}

func (m *mockFullClient) GetPrompt(ctx context.Context, name string, args map[string]string) (*PromptResult, error) {
	if m.promptRes != nil {
		return m.promptRes, nil
	}
	return &PromptResult{
		Description: "mock prompt",
		Messages: []PromptMessage{
			{Role: "user", Content: fmt.Sprintf("prompt %s with args %v", name, args)},
		},
	}, nil
}

func (m *mockFullClient) ListResources(ctx context.Context) ([]ResourceInfo, error) {
	return m.resources, nil
}

func (m *mockFullClient) ReadResource(ctx context.Context, uri string) (*ResourceContent, error) {
	if m.resContent != nil {
		return m.resContent, nil
	}
	return &ResourceContent{URI: uri, Text: "resource content for " + uri}, nil
}

// --- Tests ---

func TestListPrompts_Supported(t *testing.T) {
	c := &mockFullClient{
		prompts: []PromptInfo{
			{Name: "greet", Description: "Greeting prompt", Arguments: []PromptArgument{
				{Name: "name", Required: true},
			}},
			{Name: "summarize", Description: "Summarize text"},
		},
	}
	result, err := ListPrompts(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(result))
	}
	if result[0].Name != "greet" {
		t.Errorf("expected first prompt 'greet', got '%s'", result[0].Name)
	}
	if len(result[0].Arguments) != 1 || !result[0].Arguments[0].Required {
		t.Error("expected argument 'name' to be required")
	}
}

func TestListPrompts_NotSupported(t *testing.T) {
	c := &mockClient{}
	_, err := ListPrompts(context.Background(), c)
	if err == nil {
		t.Fatal("expected error for unsupported client")
	}
}

func TestGetPrompt_Supported(t *testing.T) {
	c := &mockFullClient{
		promptRes: &PromptResult{
			Description: "test prompt",
			Messages: []PromptMessage{
				{Role: "user", Content: "Hello, World!"},
			},
		},
	}
	result, err := GetPrompt(context.Background(), c, "greet", map[string]string{"name": "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Description != "test prompt" {
		t.Errorf("unexpected description: %s", result.Description)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "Hello, World!" {
		t.Errorf("unexpected content: %s", result.Messages[0].Content)
	}
}

func TestGetPrompt_NotSupported(t *testing.T) {
	c := &mockClient{}
	_, err := GetPrompt(context.Background(), c, "test", nil)
	if err == nil {
		t.Fatal("expected error for unsupported client")
	}
}

func TestListResources_Supported(t *testing.T) {
	c := &mockFullClient{
		resources: []ResourceInfo{
			{URI: "file:///a.txt", Name: "File A", MimeType: "text/plain"},
			{URI: "file:///b.json", Name: "File B", MimeType: "application/json"},
		},
	}
	result, err := ListResources(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result))
	}
	if result[0].URI != "file:///a.txt" {
		t.Errorf("unexpected URI: %s", result[0].URI)
	}
}

func TestListResources_NotSupported(t *testing.T) {
	c := &mockClient{}
	_, err := ListResources(context.Background(), c)
	if err == nil {
		t.Fatal("expected error for unsupported client")
	}
}

func TestReadResource_Supported(t *testing.T) {
	c := &mockFullClient{}
	result, err := ReadResource(context.Background(), c, "file:///test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result.URI != "file:///test.txt" {
		t.Errorf("unexpected URI: %s", result.URI)
	}
	if result.Text == "" {
		t.Error("expected non-empty text content")
	}
}

func TestReadResource_NotSupported(t *testing.T) {
	c := &mockClient{}
	_, err := ReadResource(context.Background(), c, "file:///test.txt")
	if err == nil {
		t.Fatal("expected error for unsupported client")
	}
}

func TestPromptsClientInterface(t *testing.T) {
	var c Client = &mockFullClient{}
	if _, ok := c.(PromptsClient); !ok {
		t.Error("mockFullClient should implement PromptsClient")
	}
	if _, ok := c.(ResourcesClient); !ok {
		t.Error("mockFullClient should implement ResourcesClient")
	}
	if _, ok := c.(SamplingClient); ok {
		t.Error("mockFullClient should NOT implement SamplingClient")
	}
}

func TestSamplingHelper_NotSupported(t *testing.T) {
	c := &mockClient{}
	_, err := CreateMessage(context.Background(), c, SamplingRequest{})
	if err == nil {
		t.Fatal("expected error for unsupported client")
	}
}
