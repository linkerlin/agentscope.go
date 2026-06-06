package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTextLoader(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(f, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	l := &TextLoader{}
	docs, err := l.Load(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].Content != "hello world" {
		t.Fatalf("unexpected content: %q", docs[0].Content)
	}
	if docs[0].Metadata["filename"] != "test.txt" {
		t.Fatalf("unexpected filename: %v", docs[0].Metadata["filename"])
	}
}

func TestTextLoader_NotFound(t *testing.T) {
	l := &TextLoader{}
	_, err := l.Load(context.Background(), "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDirLoader(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("alpha"), 0644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("beta"), 0644)
	os.WriteFile(filepath.Join(tmp, "c.log"), []byte("gamma"), 0644)

	l := &DirLoader{
		Filter: func(path string, info os.FileInfo) bool {
			return filepath.Ext(path) == ".txt"
		},
	}
	docs, err := l.Load(context.Background(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	contents := make(map[string]bool)
	for _, d := range docs {
		contents[d.Content] = true
	}
	if !contents["alpha"] || !contents["beta"] {
		t.Fatalf("unexpected contents: %v", contents)
	}
}

func TestDirLoader_ContextCancellation(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("alpha"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	l := &DirLoader{}
	_, err := l.Load(ctx, tmp)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
