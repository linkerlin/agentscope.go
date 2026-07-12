// Command agentic_memory demonstrates the agentic long-term memory middleware:
// a filesystem Markdown store whose MEMORY.md index is injected into the system
// prompt, and whose relevant topic files are surfaced as hints during reasoning.
//
// Runs offline (no API key) using the keyword selector. Replace with an
// LLM-backed selector (via WithSelector) in production.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/middleware"
)

func main() {
	dir := filepath.Join(os.TempDir(), "agentscope-agentic-demo")
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)

	store, err := middleware.NewLocalMemoryStore(dir)
	if err != nil {
		panic(err)
	}

	// Seed a couple of memory files (in production the AGENT writes these
	// itself via its file tools, following the injected instructions).
	seed := func(name, body string) {
		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	seed("user_role.md", "---\nname: role\ndescription: user is a senior Go engineer\ntype: user\n---\nThe user is a senior Go engineer who prefers terse, idiomatic code.")
	seed("feedback_testing.md", "---\nname: testing\ndescription: prefer integration tests over mocks\ntype: feedback\n---\nAlways write integration tests against a real database, never mocks.\n\nWhy: a prior incident where mocked tests passed but prod failed.\nHow to apply: when touching data-layer code.")
	os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("- [Role](user_role.md) — senior Go engineer\n- [Testing](feedback_testing.md) — real DB, not mocks\n"), 0o644)

	mw := middleware.NewAgenticMemoryMiddleware(store, dir)
	ctx := context.Background()

	// 1. System prompt gets the instructions + MEMORY.md snapshot.
	prompt, _ := mw.OnSystemPrompt(ctx, demoAgent{"Friday"}, "You are Friday, a helpful assistant.")
	fmt.Println("=== SYSTEM PROMPT (tail) ===")
	fmt.Println(prompt[len(prompt)-400:])

	// 2. Retrieval surfaces the relevant memory as a hint.
	fmt.Println("\n=== Retrieval for 'how should I test the database layer?' ===")
	files, _ := store.ListFiles(ctx)
	hits, _ := middleware.KeywordSelector(5)(ctx, "how should I test the database layer", files)
	for _, h := range hits {
		fmt.Printf("  → %s: %s\n", h.Filename, h.Description)
	}
}

type demoAgent struct{ name string }

func (a demoAgent) AgentName() string { return a.name }
