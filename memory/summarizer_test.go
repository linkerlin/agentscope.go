package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummarizerAppendToMemoryMD(t *testing.T) {
	dir := t.TempDir()
	s := &Summarizer{WorkingDir: dir}
	if err := s.AppendToMemoryMD("Notes", "- item 1"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendToMemoryMD("", "more"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "memory", "MEMORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Notes") || !strings.Contains(string(data), "more") {
		t.Fatal(string(data))
	}
}
