package file

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestReadFileTool_BinaryReturnsDataBlock(t *testing.T) {
	dir := t.TempDir()
	r := NewReadFileTool(dir)

	// Invalid UTF-8 (a tiny GIF header) triggers the DataBlock path.
	gif := []byte("GIF89a" + "\x00\xff\xfe\x00;") //nolint — deliberate binary bytes
	path := filepath.Join(dir, "logo.gif")
	if err := os.WriteFile(path, gif, 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := r.Execute(context.Background(), map[string]any{"file_path": path})
	if err != nil {
		t.Fatal(err)
	}
	var data *message.DataBlock
	for _, b := range resp.Content {
		if db, ok := b.(*message.DataBlock); ok {
			data = db
		}
	}
	if data == nil {
		t.Fatalf("expected a DataBlock for binary content, got %+v", resp.Content)
	}
	if data.Source == nil || data.Source.Type != message.SourceTypeBase64 {
		t.Fatalf("expected base64 source, got %+v", data.Source)
	}
	decoded, err := base64.StdEncoding.DecodeString(data.Source.Data)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(gif) {
		t.Fatal("base64 roundtrip mismatch")
	}
	if !strings.Contains(resp.GetTextContent(), "binary file") {
		t.Fatalf("explanation text missing: %q", resp.GetTextContent())
	}
}

func TestReadFileTool_TextUnchanged(t *testing.T) {
	dir := t.TempDir()
	r := NewReadFileTool(dir)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("plain text\nsecond line"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := r.Execute(context.Background(), map[string]any{"file_path": filepath.Join(dir, "a.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "plain text") {
		t.Fatalf("text content missing: %q", resp.GetTextContent())
	}
	for _, b := range resp.Content {
		if _, ok := b.(*message.DataBlock); ok {
			t.Fatal("text files must not produce a DataBlock")
		}
	}
}
