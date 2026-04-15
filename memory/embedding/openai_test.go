package embedding

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestOpenAIEmbedder_Dimensions(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	e := NewOpenAIEmbedder(apiKey, "text-embedding-3-small")
	ctx := context.Background()

	vec, err := e.Embed(ctx, "hello world")
	if err != nil {
		if isAuthError(err) {
			t.Skip("invalid OPENAI_API_KEY, skipping live test")
		}
		t.Fatalf("embed failed: %v", err)
	}
	if len(vec) == 0 {
		t.Fatal("expected non-empty vector")
	}

	vecs, err := e.EmbedBatch(ctx, []string{"hello", "world"})
	if err != nil {
		if isAuthError(err) {
			t.Skip("invalid OPENAI_API_KEY, skipping live test")
		}
		t.Fatalf("embed batch failed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) == 0 || len(vecs[1]) == 0 {
		t.Fatal("expected non-empty vectors in batch")
	}
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "Incorrect API key")
}

func TestOpenAIEmbedder_DefaultModel(t *testing.T) {
	e := NewOpenAIEmbedder("fake-key", "")
	if e.modelName == "" {
		t.Fatal("expected default model name")
	}
}

func TestLocalEmbedder_NotImplemented(t *testing.T) {
	e := NewLocalEmbedder("http://localhost:11434", "nomic-embed-text", 768)
	ctx := context.Background()

	_, err := e.Embed(ctx, "test")
	if err == nil {
		t.Fatal("expected error for unimplemented local embedder")
	}

	_, err = e.EmbedBatch(ctx, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for unimplemented local embedder batch")
	}
}
