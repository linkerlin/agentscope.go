package ollama

import (
	"testing"

	"github.com/linkerlin/agentscope.go/formatter"
)

func TestOllamaBuilder_Formatter(t *testing.T) {
	f := formatter.NewOllamaFormatter()
	m, err := NewBuilder().
		Formatter(f).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if m.ModelName() != "llama3.2" {
		t.Fatalf("unexpected model name: %s", m.ModelName())
	}
}

func TestOllamaBuilder_Default(t *testing.T) {
	m, err := NewBuilder().Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if m.ModelName() != "llama3.2" {
		t.Fatalf("unexpected model name: %s", m.ModelName())
	}
}
