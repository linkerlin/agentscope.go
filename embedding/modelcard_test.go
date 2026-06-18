package embedding_test

import (
	"testing"

	"github.com/linkerlin/agentscope.go/embedding"
)

func TestListModelCards(t *testing.T) {
	cards, err := embedding.ListModelCards()
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) < 6 {
		t.Fatalf("expected at least 6 embedding cards, got %d", len(cards))
	}
	seen := map[string]bool{}
	for _, c := range cards {
		seen[c.ID] = true
		if c.Provider == "" || c.Model == "" {
			t.Fatalf("card missing provider/model: %+v", c)
		}
	}
	for _, want := range []string{
		"text-embedding-3-large", "text-embedding-3-small",
		"gemini-embedding-001",
		"text-embedding-v3", "text-embedding-v4", "multimodal-embedding-one-peace-v1",
		"nomic-embed-text",
	} {
		if !seen[want] {
			t.Fatalf("missing embedding card: %s", want)
		}
	}
}

func TestFindModelCard(t *testing.T) {
	c := embedding.FindModelCard("text-embedding-3-large")
	if c == nil || c.Dimensions != 3072 {
		t.Fatalf("find card failed: %+v", c)
	}
	if embedding.FindModelCard("nope") != nil {
		t.Fatal("expected nil for unknown card")
	}
}

func TestModelCardsByProvider(t *testing.T) {
	groups, err := embedding.ModelCardsByProvider()
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	for _, prov := range []string{"openai", "gemini", "dashscope", "ollama"} {
		if len(groups[prov]) == 0 {
			t.Fatalf("provider %s has no cards", prov)
		}
	}
	// DashScope should include the multimodal card.
	var hasMultimodal bool
	for _, c := range groups["dashscope"] {
		for _, mod := range c.Modalities {
			if mod == "image" {
				hasMultimodal = true
			}
		}
	}
	if !hasMultimodal {
		t.Fatal("expected a DashScope multimodal embedding card")
	}
}
