package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

func TestToolResultCompactorTruncatesLarge(t *testing.T) {
	dir := t.TempDir()
	c := NewToolResultCompactor(dir, 100, 50, 3)
	long := strings.Repeat("x", 200)
	tr := message.NewToolResultBlock("tu1", []message.ContentBlock{
		message.NewTextBlock(long),
	}, false)
	msg := message.NewMsg().Role(message.RoleUser).Content(tr).Build()
	out, err := c.Compact([]*message.Msg{msg}, 1)
	if err != nil || len(out) != 1 {
		t.Fatal(err, len(out))
	}
	found := false
	for _, b := range out[0].Content {
		if trb, ok := b.(*message.ToolResultBlock); ok {
			for _, c := range trb.Content {
				if tb, ok := c.(*message.TextBlock); ok && strings.Contains(tb.Text, "truncated") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("expected truncation marker in tool result")
	}
}

func TestToolResultCompactorPurgeExpired(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	c := NewToolResultCompactor(dir, 100, 50, 1)
	if err := c.PurgeExpired(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldFile); err == nil {
		t.Fatal("old file should be removed")
	}
}

func TestToolResultCompactorNil(t *testing.T) {
	var c *ToolResultCompactor
	out, err := c.Compact([]*message.Msg{message.NewMsg().Role(message.RoleUser).Build()}, 1)
	if err != nil || out == nil {
		t.Fatal(err)
	}
	if c.PurgeExpired() != nil {
		t.Fatal()
	}
}
