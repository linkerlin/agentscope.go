package blob

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalBlobStore_PutGetDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalBlobStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	uri, err := s.Put(ctx, "kb1/doc.txt", strings.NewReader("hello blob"))
	if err != nil {
		t.Fatal(err)
	}
	if uri != "local://kb1/doc.txt" {
		t.Fatalf("unexpected uri %q", uri)
	}

	rc, err := s.Get(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()
	if string(data) != "hello blob" {
		t.Fatalf("got %q", data)
	}

	if err := s.Delete(ctx, uri); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, uri); !os.IsNotExist(err) {
		t.Fatalf("expected not-exist after delete, got %v", err)
	}
}

func TestLocalBlobStore_DeleteMissingNotError(t *testing.T) {
	s, _ := NewLocalBlobStore(t.TempDir())
	if err := s.Delete(context.Background(), "local://nope"); err != nil {
		t.Fatalf("deleting missing blob should not error, got %v", err)
	}
}

func TestLocalBlobStore_UnsupportedURI(t *testing.T) {
	s, _ := NewLocalBlobStore(t.TempDir())
	if _, err := s.Get(context.Background(), "s3://bucket/key"); err == nil {
		t.Fatal("expected error for unsupported uri")
	}
}

func TestLocalBlobStore_CreatesRootDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "blobroot")
	s, err := NewLocalBlobStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("root dir not created: %v", err)
	}
	// round-trip
	uri, _ := s.Put(context.Background(), "k", bytes.NewReader([]byte("x")))
	rc, _ := s.Get(context.Background(), uri)
	defer rc.Close()
}
