package embedding

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed cards/*.yaml
var embeddingCardsFS embed.FS

// ModelCard describes an embedding model configuration (mirrors Python
// agentscope's per-provider embedding model cards, #1852). It enables
// Studio-driven dynamic forms and self-documenting provider catalogs.
type ModelCard struct {
	// ID is the unique card identifier, e.g. "text-embedding-3-large".
	ID string `yaml:"id" json:"id"`
	// Provider is the backend provider: "openai", "ollama", "gemini", "dashscope".
	Provider string `yaml:"provider" json:"provider"`
	// DisplayName is a human-friendly name.
	DisplayName string `yaml:"display_name" json:"display_name"`
	// Model is the wire model name passed to the API.
	Model string `yaml:"model" json:"model"`
	// Dimensions is the embedding vector dimensionality (0 = provider-default/configurable).
	Dimensions int `yaml:"dimensions" json:"dimensions"`
	// MaxInputTokens is the maximum input context length.
	MaxInputTokens int `yaml:"max_input_tokens" json:"max_input_tokens"`
	// Modalities lists supported input modalities: "text", "image".
	Modalities []string `yaml:"modalities" json:"modalities"`
	// Default marks the recommended default for a provider.
	Default bool `yaml:"default" json:"default"`
}

// ListModelCards loads all embedded embedding model cards. Malformed cards are
// skipped (not appended) so a single bad entry never breaks discovery.
func ListModelCards() ([]*ModelCard, error) {
	entries, err := embeddingCardsFS.ReadDir("cards")
	if err != nil {
		return nil, fmt.Errorf("embedding modelcard: read embedded cards: %w", err)
	}
	var cards []*ModelCard
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, err := embeddingCardsFS.ReadFile("cards/" + name)
		if err != nil {
			continue
		}
		var card ModelCard
		if err := yaml.Unmarshal(data, &card); err != nil {
			continue
		}
		if card.ID == "" {
			continue
		}
		cards = append(cards, &card)
	}
	return cards, nil
}

// FindModelCard returns the first embedded card whose ID matches, or nil.
func FindModelCard(id string) *ModelCard {
	cards, err := ListModelCards()
	if err != nil {
		return nil
	}
	for _, c := range cards {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// ModelCardsByProvider groups cards by Provider.
func ModelCardsByProvider() (map[string][]*ModelCard, error) {
	cards, err := ListModelCards()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]*ModelCard)
	for _, c := range cards {
		out[c.Provider] = append(out[c.Provider], c)
	}
	return out, nil
}
