// Package builtin provides a filesystem-backed demo hub: card catalogs are
// loaded from JSON files, so a marketplace can be published as a plain JSON
// tree (or served by any static host). Implements hub.Hub.
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/hub"
)

// FSHub is a hub whose catalog comes from two JSON files:
//
//	mcps.json   — an array of hub.MCPCard
//	skills.json — an array of hub.SkillCard
type FSHub struct {
	id, name    string
	description string
	mcpCards    []hub.MCPCard
	skillCards  []hub.SkillCard
}

// NewFSHub loads the catalog from dir (mcps.json + skills.json, both
// optional — a missing file means an empty catalog for that kind).
func NewFSHub(dir, id, name, description string) (*FSHub, error) {
	h := &FSHub{id: id, name: name, description: description}
	var err error
	h.mcpCards, err = loadCards[hub.MCPCard](filepath.Join(dir, "mcps.json"))
	if err != nil {
		return nil, err
	}
	h.skillCards, err = loadCards[hub.SkillCard](filepath.Join(dir, "skills.json"))
	if err != nil {
		return nil, err
	}
	return h, nil
}

func loadCards[T any](path string) ([]T, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // empty catalog, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("hub: read %s: %w", path, err)
	}
	var out []T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("hub: parse %s: %w", path, err)
	}
	return out, nil
}

// ID returns the hub identifier.
func (h *FSHub) ID() string { return h.id }

// DisplayName returns the user-facing name.
func (h *FSHub) DisplayName() string { return h.name }

// ListMCPCards returns a filtered, paginated page of MCP cards.
func (h *FSHub) ListMCPCards(ctx context.Context, query string, cursor, limit int) ([]hub.MCPCard, int, error) {
	filtered := hub.FilterCards(h.mcpCards, query,
		func(c hub.MCPCard) string { return c.Name },
		func(c hub.MCPCard) string { return c.Description })
	page, next := hub.Page(filtered, cursor, limit)
	return page, next, nil
}

// ListSkillCards returns a filtered, paginated page of skill cards.
func (h *FSHub) ListSkillCards(ctx context.Context, query string, cursor, limit int) ([]hub.SkillCard, int, error) {
	filtered := hub.FilterCards(h.skillCards, query,
		func(c hub.SkillCard) string { return c.Name },
		func(c hub.SkillCard) string { return c.Description })
	page, next := hub.Page(filtered, cursor, limit)
	return page, next, nil
}
