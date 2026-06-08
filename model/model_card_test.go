package model

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadModelCardsFromDir_Count(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "cards")
	cards, err := LoadModelCardsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) < 33 {
		t.Fatalf("expected at least 33 model cards, got %d", len(cards))
	}
	providers := map[string]int{}
	for _, c := range cards {
		providers[c.Provider]++
		if c.ID == "" || c.DisplayName == "" {
			t.Fatalf("invalid card: %#v", c)
		}
	}
	for _, p := range []string{"anthropic", "dashscope", "deepseek", "gemini", "moonshot", "ollama", "openai", "openai_response", "xai"} {
		if providers[p] == 0 {
			t.Fatalf("missing provider %q in cards", p)
		}
	}
}
