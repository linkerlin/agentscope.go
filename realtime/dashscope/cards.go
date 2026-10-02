// realtime/dashscope/cards.go — embedded realtime model cards (19.4).
package dashscope

import (
	"embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/linkerlin/agentscope.go/realtime"
)

//go:embed cards/*.yaml
var cardsFS embed.FS

// ListModelCards loads every embedded DashScope realtime card (sorted by id).
// Malformed cards are skipped (a card is metadata, never a hard dependency).
func ListModelCards() ([]realtime.ModelCard, error) {
	entries, err := cardsFS.ReadDir("cards")
	if err != nil {
		return nil, fmt.Errorf("dashscope realtime cards: %w", err)
	}
	var cards []realtime.ModelCard
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := cardsFS.ReadFile("cards/" + e.Name())
		if err != nil {
			continue
		}
		var c realtime.ModelCard
		if err := yaml.Unmarshal(data, &c); err != nil {
			continue
		}
		cards = append(cards, c)
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
	return cards, nil
}

// cardByModel finds the embedded card whose wire model matches; ok=false
// when absent (the caller fills a conservative fallback card).
func cardByModel(model string) (realtime.ModelCard, bool) {
	cards, err := ListModelCards()
	if err != nil {
		return realtime.ModelCard{}, false
	}
	for _, c := range cards {
		if c.Model == model {
			return c, true
		}
	}
	return realtime.ModelCard{}, false
}
