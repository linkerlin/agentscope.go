package vllm

import (
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func TestVLLMBuilder_Defaults(t *testing.T) {
	b := Builder("not-needed")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != "" {
		t.Fatalf("expected empty default model, got %s", m.ModelName())
	}
}

func TestVLLMBuilder_CustomModel(t *testing.T) {
	b := Builder("dummy").ModelName("meta-llama/Llama-2-7b-chat-hf")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != "meta-llama/Llama-2-7b-chat-hf" {
		t.Fatalf("expected custom model, got %s", m.ModelName())
	}
}

func TestVLLMBuilder_CustomBaseURL(t *testing.T) {
	b := Builder("dummy").BaseURL("http://vllm-cluster.internal:8000/v1")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != "" {
		t.Fatalf("expected empty model, got %s", m.ModelName())
	}
}

func TestVLLMBuilder_EmptyAPIKey(t *testing.T) {
	// vLLM often does not require an API key; empty key should still build
	b := Builder("").APIKey("")
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error for empty API key because openai builder requires it")
	}
}

func TestVLLMBuilder_Retry(t *testing.T) {
	b := Builder("dummy").Retry(3, 500*time.Millisecond)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVLLMBuilder_Formatter(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	b := Builder("dummy").Formatter(f)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVLLMBuilder_Chain(t *testing.T) {
	b := Builder("dummy").
		ModelName("meta-llama/Llama-2-7b-chat-hf").
		BaseURL("http://vllm-cluster.internal:8000/v1").
		Retry(2, 100*time.Millisecond)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != "meta-llama/Llama-2-7b-chat-hf" {
		t.Fatalf("expected model, got %s", m.ModelName())
	}
}

// compile-time check
var _ = (*openai.OpenAIChatModel)(nil)
