package deepseek

import (
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func TestDeepSeekBuilder_Defaults(t *testing.T) {
	b := Builder("test-key")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelChat {
		t.Fatalf("expected default model %s, got %s", ModelChat, m.ModelName())
	}
}

func TestDeepSeekBuilder_CustomModel(t *testing.T) {
	b := Builder("test-key").ModelName(ModelCoder)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelCoder {
		t.Fatalf("expected model %s, got %s", ModelCoder, m.ModelName())
	}
}

func TestDeepSeekBuilder_CustomBaseURL(t *testing.T) {
	b := Builder("test-key").BaseURL("https://proxy.example.com/v1")
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	// ModelName is a getter exposed by OpenAIChatModel
	if m.ModelName() != ModelChat {
		t.Fatalf("expected model %s, got %s", ModelChat, m.ModelName())
	}
}

func TestDeepSeekBuilder_EmptyAPIKey(t *testing.T) {
	b := Builder("").APIKey("")
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestDeepSeekBuilder_Retry(t *testing.T) {
	b := Builder("test-key").Retry(3, 500*time.Millisecond)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeepSeekBuilder_Formatter(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	b := Builder("test-key").Formatter(f)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeepSeekBuilder_Chain(t *testing.T) {
	b := Builder("test-key").
		ModelName(ModelReason).
		BaseURL("https://proxy.example.com/v1").
		Retry(2, 100*time.Millisecond)
	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelReason {
		t.Fatalf("expected model %s, got %s", ModelReason, m.ModelName())
	}
}

// compile-time check: deepseek builder produces an openai-compatible model
var _ = (*openai.OpenAIChatModel)(nil)
