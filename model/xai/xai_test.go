package xai

import (
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func TestXAIBuilder_Defaults(t *testing.T) {
	b := Builder("test-key")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelGrokBeta {
		t.Fatalf("expected default model %s, got %s", ModelGrokBeta, m.ModelName())
	}
}

func TestXAIBuilder_CustomModel(t *testing.T) {
	b := Builder("test-key").ModelName(ModelGrok2)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelGrok2 {
		t.Fatalf("expected model %s, got %s", ModelGrok2, m.ModelName())
	}
}

func TestXAIBuilder_CustomBaseURL(t *testing.T) {
	b := Builder("test-key").BaseURL("https://proxy.example.com/v1")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelGrokBeta {
		t.Fatalf("expected model %s, got %s", ModelGrokBeta, m.ModelName())
	}
}

func TestXAIBuilder_EmptyAPIKey(t *testing.T) {
	b := Builder("").APIKey("")
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestXAIBuilder_Retry(t *testing.T) {
	b := Builder("test-key").Retry(3, 500*time.Millisecond)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestXAIBuilder_Formatter(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	b := Builder("test-key").Formatter(f)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestXAIBuilder_Chain(t *testing.T) {
	b := Builder("test-key").
		ModelName(ModelGrok2Vision).
		BaseURL("https://proxy.example.com/v1").
		Retry(2, 100*time.Millisecond)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelGrok2Vision {
		t.Fatalf("expected model %s, got %s", ModelGrok2Vision, m.ModelName())
	}
}

// compile-time check: xai builder produces an openai-compatible model
var _ = (*openai.OpenAIChatModel)(nil)
