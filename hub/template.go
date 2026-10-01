// hub/template.go — configuration-template validation and filling (18.7):
// marketplace MCP cards ship specs with ${VAR} placeholders; installing one
// means proving every required value exists BEFORE any server process is
// spawned, and expanding the placeholders into a concrete spec.
package hub

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	mcpserver "github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// placeholderPattern matches ${VAR} references (braced only — a bare $FOO
// collides with shell conventions and is deliberately not a placeholder).
var placeholderPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// extractPlaceholders returns every distinct ${VAR} name referenced anywhere
// in the spec's string fields, sorted.
func extractPlaceholders(spec mcpserver.ServerSpec) []string {
	seen := map[string]bool{}
	add := func(s string) {
		for _, m := range placeholderPattern.FindAllStringSubmatch(s, -1) {
			seen[m[1]] = true
		}
	}
	add(spec.Command)
	add(spec.URL)
	for _, a := range spec.Args {
		add(a)
	}
	for _, v := range spec.Env {
		add(v)
	}
	for _, v := range spec.Headers {
		add(v)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ValidateMCPCard checks that every ${VAR} the card's spec references has a
// value. The card's RequiredEnv (when declared) must also be provided even
// if the spec only references it in documented-but-optional positions.
// Returns the sorted list of missing variables (empty = valid).
func ValidateMCPCard(card MCPCard, values map[string]string) []string {
	missing := map[string]bool{}
	for _, name := range extractPlaceholders(card.Spec) {
		if _, ok := values[name]; !ok {
			missing[name] = true
		}
	}
	for _, name := range card.RequiredEnv {
		if _, ok := values[name]; !ok {
			missing[name] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	out := make([]string, 0, len(missing))
	for k := range missing {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ExpandSpec returns a copy of spec with every ${VAR} replaced by its value.
// Callers must ValidateMCPCard first; unreplaced placeholders are left
// verbatim (never half-filled) so a misconfigured install fails to connect
// loudly rather than silently shipping "${TOKEN}" to a server.
func ExpandSpec(spec mcpserver.ServerSpec, values map[string]string) mcpserver.ServerSpec {
	expand := func(s string) string {
		if !strings.Contains(s, "${") {
			return s
		}
		return placeholderPattern.ReplaceAllStringFunc(s, func(m string) string {
			name := m[2 : len(m)-1]
			if v, ok := values[name]; ok {
				return v
			}
			return m // leave verbatim when unvalued
		})
	}
	out := spec
	out.Command = expand(spec.Command)
	out.URL = expand(spec.URL)
	out.Args = make([]string, len(spec.Args))
	for i, a := range spec.Args {
		out.Args[i] = expand(a)
	}
	out.Env = make(map[string]string, len(spec.Env))
	for k, v := range spec.Env {
		out.Env[k] = expand(v)
	}
	out.Headers = make(map[string]string, len(spec.Headers))
	for k, v := range spec.Headers {
		out.Headers[k] = expand(v)
	}
	return out
}

// InstallMCPsWithValues validates every card's template against values
// (failing BEFORE any connection attempt, listing all missing variables),
// expands the placeholders, and connects via the same resilient loader as
// InstallMCPs.
func InstallMCPsWithValues(ctx context.Context, cards []MCPCard, values map[string]string) (*mcpserver.Manager, []mcpserver.ConnectResult, error) {
	var missingByCard []string
	expanded := make([]MCPCard, 0, len(cards))
	for _, c := range cards {
		if missing := ValidateMCPCard(c, values); len(missing) > 0 {
			missingByCard = append(missingByCard, fmt.Sprintf("%s: missing %s", c.ID, strings.Join(missing, ", ")))
			continue
		}
		nc := c
		nc.Spec = ExpandSpec(c.Spec, values)
		expanded = append(expanded, nc)
	}
	if len(missingByCard) > 0 {
		return nil, nil, fmt.Errorf("hub: template validation failed: %s", strings.Join(missingByCard, "; "))
	}
	mgr, results := InstallMCPs(ctx, expanded)
	return mgr, results, nil
}
