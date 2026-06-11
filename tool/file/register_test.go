package file

import (
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
)

func TestRegisterAll_ReadOnly(t *testing.T) {
	tools := RegisterAll("/tmp", true)
	if len(tools) != 3 {
		t.Fatalf("expected 3 read-only tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Name()] = true
	}
	for _, want := range []string{"view_text_file", "glob", "grep"} {
		if !names[want] {
			t.Fatalf("expected tool %q not found", want)
		}
	}
}

func TestRegisterAll_All(t *testing.T) {
	tools := RegisterAll("/tmp", false)
	if len(tools) != 6 {
		var got []string
		for _, tl := range tools {
			got = append(got, tl.Name())
		}
		t.Fatalf("expected 6 tools, got %d: %v", len(tools), got)
	}
}

func TestRegisterAll_WriteToolsOmitted(t *testing.T) {
	tools := RegisterAll("/tmp", true)
	for _, tl := range tools {
		name := tl.Name()
		if name == "write_text_file" || name == "edit_text_file" || name == "insert_text_file" {
			t.Fatalf("write tool %q should not be in read-only set", name)
		}
	}
}

var _ tool.Tool = NewReadFileTool("/tmp")
