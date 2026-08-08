// Package hub provides a marketplace abstraction for installing MCP servers
// and skills from registries, mirroring Python agentscope's Hub system
// (HubBase / MCPHubBase / SkillHubBase, #2197). A hub exposes a browsable,
// paginated catalog of MCPCard / SkillCard entries; installers turn a card
// into a connected MCP manager or an unpacked skill directory.
package hub

import (
	"context"
	"strings"

	mcpserver "github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// Card is the common metadata of any marketplace entry.
type Card struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	IconURL     string   `json:"icon_url,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// MCPCard is a marketplace entry for an MCP server. Spec carries the exact
// connection configuration (a toolkit/mcp.ServerSpec, with $VAR placeholders
// expanded at install time).
type MCPCard struct {
	Card
	Spec mcpserver.ServerSpec `json:"spec"`
}

// SkillCard is a marketplace entry for a skill. ArchiveURL points to the
// downloadable skill archive (zip/tar/tar.gz).
type SkillCard struct {
	Card
	ArchiveURL string `json:"archive_url"`
}

// Hub is a browsable marketplace. List methods return the page slice plus the
// next cursor (-1 when there are no more pages). Implementations must be safe
// for concurrent use.
type Hub interface {
	// ID returns the stable hub identifier (used in HTTP routes).
	ID() string
	// DisplayName returns the user-facing hub name.
	DisplayName() string
	// ListMCPCards returns a page of MCP cards matching query ("" = all).
	ListMCPCards(ctx context.Context, query string, cursor, limit int) ([]MCPCard, int, error)
	// ListSkillCards returns a page of skill cards matching query ("" = all).
	ListSkillCards(ctx context.Context, query string, cursor, limit int) ([]SkillCard, int, error)
}

// Page is a cursor-based pagination helper: returns the slice for the page and
// the next cursor (-1 if this was the last page).
func Page[T any](all []T, cursor, limit int) ([]T, int) {
	if limit <= 0 {
		limit = 20
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(all) {
		return nil, -1
	}
	end := cursor + limit
	if end > len(all) {
		end = len(all)
	}
	next := end
	if end >= len(all) {
		next = -1
	}
	return all[cursor:end], next
}

// matchesQuery filters entries by case-insensitive substring on name + description.
func matchesQuery(name, description, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(name), q) ||
		strings.Contains(strings.ToLower(description), q)
}

// FilterCards returns cards matching the query.
func FilterCards[T any](cards []T, query string, name func(T) string, desc func(T) string) []T {
	if query == "" {
		return cards
	}
	out := make([]T, 0, len(cards))
	for _, c := range cards {
		if matchesQuery(name(c), desc(c), query) {
			out = append(out, c)
		}
	}
	return out
}
