package model

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelCard describes a model provider configuration (PyV2 model/_model_card.py).
// ModelPricing is the optional per-token price carried by a ModelCard,
// in dollars per million tokens (consumed by CostRouter, 16.3).
type ModelPricing struct {
	InputPerMTok  float64 `yaml:"input_per_mtok" json:"input_per_mtok"`
	OutputPerMTok float64 `yaml:"output_per_mtok" json:"output_per_mtok"`
}

type ModelCard struct {
	ID              string         `yaml:"id" json:"id"`
	Provider        string         `yaml:"provider" json:"provider"`
	DisplayName     string         `yaml:"display_name" json:"display_name"`
	ContextSize     int            `yaml:"context_size" json:"context_size"`
	OutputSize      int            `yaml:"output_size" json:"output_size"`
	InputTypes      []string       `yaml:"input_types" json:"input_types"`
	OutputTypes     []string       `yaml:"output_types" json:"output_types"`
	ParameterSchema map[string]any `yaml:"parameter_schema" json:"parameter_schema,omitempty"`
	Deprecated      bool           `yaml:"deprecated" json:"deprecated"`
	// Tier optionally labels the cost/perf class (e.g. flagship/standard/lite).
	Tier string `yaml:"tier,omitempty" json:"tier,omitempty"`
	// Pricing optionally carries per-token prices for cost routing.
	Pricing *ModelPricing `yaml:"pricing,omitempty" json:"pricing,omitempty"`
}

// LoadModelCard reads a single YAML model card from disk.
func LoadModelCard(path string) (*ModelCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var card ModelCard
	if err := yaml.Unmarshal(data, &card); err != nil {
		return nil, err
	}
	if card.ID == "" {
		return nil, fmt.Errorf("model card: missing id in %s", path)
	}
	return &card, nil
}

// LoadModelCardsFromDir loads all *.yaml / *.yml files in a directory.
func LoadModelCardsFromDir(dir string) ([]*ModelCard, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var cards []*ModelCard
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 {
			continue
		}
		ext := name[len(name)-5:]
		if ext != ".yaml" && ext != ".yml" && name[len(name)-4:] != ".yml" {
			continue
		}
		card, err := LoadModelCard(dir + string(os.PathSeparator) + name)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}
