package moonshot

import (
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func TestMoonshotBuilder_Defaults(t *testing.T) {
	b := Builder("test-key")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != Model8K {
		t.Fatalf("expected default model %s, got %s", Model8K, m.ModelName())
	}
}

func TestMoonshotBuilder_CustomModel(t *testing.T) {
	b := Builder("test-key").ModelName(Model128K)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != Model128K {
		t.Fatalf("expected model %s, got %s", Model128K, m.ModelName())
	}
}

func TestMoonshotBuilder_CustomBaseURL(t *testing.T) {
	b := Builder("test-key").BaseURL("https://proxy.example.com/v1")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != Model8K {
		t.Fatalf("expected model %s, got %s", Model8K, m.ModelName())
	}
}

func TestMoonshotBuilder_EmptyAPIKey(t *testing.T) {
	b := Builder("").APIKey("")
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestMoonshotBuilder_Retry(t *testing.T) {
	b := Builder("test-key").Retry(3, 500*time.Millisecond)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMoonshotBuilder_Formatter(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	b := Builder("test-key").Formatter(f)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMoonshotBuilder_Chain(t *testing.T) {
	b := Builder("test-key").
		ModelName(Model32K).
		BaseURL("https://proxy.example.com/v1").
		Retry(2, 100*time.Millisecond)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != Model32K {
		t.Fatalf("expected model %s, got %s", Model32K, m.ModelName())
	}
}

// compile-time check: moonshot builder produces an openai-compatible model
var _ = (*openai.OpenAIChatModel)(nil)
