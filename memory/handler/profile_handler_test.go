package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileHandler(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := NewProfileHandler(dir)

	// Add
	profile := map[string]any{"preference": "concise", "language": "zh"}
	if err := h.AddProfile(ctx, "alice", profile); err != nil {
		t.Fatal(err)
	}

	// Read
	got, err := h.ReadAllProfiles(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got["preference"] != "concise" {
		t.Fatalf("unexpected preference: %v", got["preference"])
	}

	// Update
	if err := h.UpdateProfile(ctx, "alice", map[string]any{"language": "en"}); err != nil {
		t.Fatal(err)
	}
	got2, _ := h.ReadAllProfiles(ctx, "alice")
	if got2["preference"] != "concise" || got2["language"] != "en" {
		t.Fatalf("unexpected profile after update: %v", got2)
	}

	// Read non-existing
	got3, err := h.ReadAllProfiles(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(got3) != 0 {
		t.Fatalf("expected empty profile for bob, got %v", got3)
	}

	path := filepath.Join(dir, "alice.json")
	_ = os.Remove(path)
}
