package tts

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed cards/*.yaml
var cardsFS embed.FS

// ModelCard describes a TTS model configuration (mirrors Python
// tts/_tts_model_card.py).
type ModelCard struct {
	// ID is the unique model identifier, e.g. "cosyvoice-v1".
	ID string `yaml:"id" json:"id"`
	// Provider is the backend provider, e.g. "dashscope", "openai".
	Provider string `yaml:"provider" json:"provider"`
	// DisplayName is a human-friendly name.
	DisplayName string `yaml:"display_name" json:"display_name"`
	// Model is the wire model name passed to the API.
	Model string `yaml:"model" json:"model"`
	// DefaultVoice is the default speaker voice.
	DefaultVoice string `yaml:"default_voice" json:"default_voice"`
	// Formats lists supported audio formats.
	Formats []string `yaml:"formats" json:"formats"`
	// Realtime marks streaming-input (push-based) models.
	Realtime bool `yaml:"realtime" json:"realtime"`
	// Multilingual indicates multi-language support.
	Multilingual bool `yaml:"multilingual" json:"multilingual"`
}

// ListModelCards loads all embedded TTS model cards. Cards with a .yaml or
// .yml extension under cards/ are parsed; malformed cards are skipped with a
// best-effort error collected into the returned slice's absence (a nil card is
// not appended).
func ListModelCards() ([]*ModelCard, error) {
	entries, err := cardsFS.ReadDir("cards")
	if err != nil {
		return nil, fmt.Errorf("tts modelcard: read embedded cards: %w", err)
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
		data, err := cardsFS.ReadFile("cards/" + name)
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
