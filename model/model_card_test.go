package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadModelCard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpt4.yaml")
	content := "id: gpt-4\nprovider: openai\ncontext_size: 128000\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	card, err := LoadModelCard(path)
	if err != nil {
		t.Fatal(err)
	}
	if card.ID != "gpt-4" || card.ContextSize != 128000 {
		t.Fatalf("unexpected card: %+v", card)
	}
}
