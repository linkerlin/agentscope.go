package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

func writeJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewFSHub_LoadsCards(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "mcps.json", `[
	  {"id":"filesystem","name":"Filesystem","description":"local fs","spec":{"name":"filesystem","command":"npx"}},
	  {"id":"github","name":"GitHub","description":"github api","spec":{"name":"github","command":"npx"}}
	]`)
	writeJSON(t, dir, "skills.json", `[
	  {"id":"code-review","name":"Code Review","description":"review patches","archive_url":"https://example.com/cr.zip"}
	]`)

	h, err := NewFSHub(dir, "demo", "Demo Hub", "")
	if err != nil {
		t.Fatal(err)
	}
	if h.ID() != "demo" {
		t.Fatalf("id = %q", h.ID())
	}
	mcps, next, err := h.ListMCPCards(context.Background(), "", 0, 10)
	if err != nil || len(mcps) != 2 || next != -1 {
		t.Fatalf("mcps: %d next=%d err=%v", len(mcps), next, err)
	}
	if mcps[0].Spec.Command != "npx" {
		t.Fatalf("spec not loaded: %+v", mcps[0].Spec)
	}
	skills, _, _ := h.ListSkillCards(context.Background(), "", 0, 10)
	if len(skills) != 1 || skills[0].ArchiveURL == "" {
		t.Fatalf("skills: %+v", skills)
	}
}

func TestNewFSHub_MissingFilesEmpty(t *testing.T) {
	h, err := NewFSHub(t.TempDir(), "empty", "Empty", "")
	if err != nil {
		t.Fatal(err)
	}
	mcps, _, _ := h.ListMCPCards(context.Background(), "", 0, 10)
	if len(mcps) != 0 {
		t.Fatal("expected empty mcp catalog")
	}
}

func TestNewFSHub_PaginationAndFilter(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "mcps.json", `[
	  {"id":"a","name":"alpha","description":"first"},
	  {"id":"b","name":"beta","description":"second"},
	  {"id":"c","name":"gamma","description":"third"}
	]`)
	h, _ := NewFSHub(dir, "h", "H", "")
	// page size 2
	p1, next, _ := h.ListMCPCards(context.Background(), "", 0, 2)
	if len(p1) != 2 || next != 2 {
		t.Fatalf("p1: %d next=%d", len(p1), next)
	}
	// filter
	pf, _, _ := h.ListMCPCards(context.Background(), "beta", 0, 10)
	if len(pf) != 1 || pf[0].ID != "b" {
		t.Fatalf("filtered: %+v", pf)
	}
}

func TestNewFSHub_BadJSON(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "mcps.json", `not json`)
	if _, err := NewFSHub(dir, "h", "H", ""); err == nil {
		t.Fatal("expected error for bad json")
	}
}

// compile-time assertion
var _ hub.Hub = (*FSHub)(nil)

// ensure toolkit/mcp is referenced (spec struct used in fixture above)
var _ = mcp.ServerSpec{}
