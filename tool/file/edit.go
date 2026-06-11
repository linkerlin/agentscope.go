package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/workspace"
)

// EditFileTool performs exact string replacements in files.
type EditFileTool struct {
	baseDir string
	ws      workspace.Workspace
}

// NewEditFileTool creates a new EditFileTool with optional baseDir restriction.
func NewEditFileTool(baseDir string) *EditFileTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &EditFileTool{baseDir: baseDir}
}

// WithWorkspace binds the tool to a workspace for sandboxed execution.
func (e *EditFileTool) WithWorkspace(ws workspace.Workspace) *EditFileTool {
	e.ws = ws
	return e
}

// Name returns the tool name.
func (e *EditFileTool) Name() string { return "edit_text_file" }

// Description returns the tool description.
func (e *EditFileTool) Description() string {
	return `Performs exact string replacements in files.

Usage:
- You must use the view_text_file tool at least once before editing.
- Preserve exact indentation (tabs/spaces) when specifying old_string.
- The edit will FAIL if old_string is not unique (unless replace_all=true).`
}

// Spec returns the JSON schema for the tool parameters.
func (e *EditFileTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        e.Name(),
		Description: e.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The target file path",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "The exact string to replace. Must match exactly including whitespace and indentation.",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "The string to replace old_string with.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "If true, replace all occurrences. If false (default), only replace if there is exactly one occurrence.",
				},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		},
	}
}

// Execute runs the edit.
func (e *EditFileTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	filePath, _ := input["file_path"].(string)
	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)
	replaceAll, _ := input["replace_all"].(bool)

	if oldStr == "" {
		return nil, fmt.Errorf("old_string cannot be empty")
	}

	path, err := validatePath(filePath, e.baseDir)
	if err != nil {
		return nil, err
	}

	// Read file content
	readFile := func() ([]byte, error) {
		if e.ws != nil {
			return e.ws.ReadFile(ctx, path)
		}
		return os.ReadFile(path)
	}
	writeFile := func(data []byte) error {
		if e.ws != nil {
			return e.ws.WriteFile(ctx, path, data, 0o644)
		}
		return os.WriteFile(path, data, 0o600) //nolint:gosec // G306: edit file, use 0600 for safety
	}

	data, err := readFile()
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}
	content := string(data)

	if oldStr == newStr {
		return nil, fmt.Errorf("old_string and new_string are identical. No changes to make")
	}

	occurrences := strings.Count(content, oldStr)
	if occurrences == 0 {
		return nil, fmt.Errorf("old_string not found in %s", filePath)
	}
	if occurrences > 1 && !replaceAll {
		return nil, fmt.Errorf("old_string appears %d times in %s. Set replace_all=true to replace all occurrences, or make old_string more specific", occurrences, filePath)
	}

	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		updated = strings.Replace(content, oldStr, newStr, 1)
	}

	if err := writeFile([]byte(updated)); err != nil {
		return nil, fmt.Errorf("error writing file: %w", err)
	}

	replacementMsg := fmt.Sprintf("%d occurrence(s)", occurrences)
	if replaceAll {
		replacementMsg = fmt.Sprintf("all %d occurrences", occurrences)
	} else {
		replacementMsg = "1 occurrence"
	}

	return tool.NewTextResponse(fmt.Sprintf("Successfully replaced %s in %s", replacementMsg, filePath)), nil
}

// IsReadOnly returns false because EditFileTool modifies files.
func (e *EditFileTool) IsReadOnly() bool { return false }
