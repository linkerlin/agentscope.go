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

// WriteFileTool provides file writing capabilities.
type WriteFileTool struct {
	baseDir string
	ws      workspace.Workspace
}

// NewWriteFileTool creates a new WriteFileTool with optional baseDir restriction.
func NewWriteFileTool(baseDir string) *WriteFileTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &WriteFileTool{baseDir: baseDir}
}

// WithWorkspace binds the tool to a workspace for sandboxed execution.
func (w *WriteFileTool) WithWorkspace(ws workspace.Workspace) *WriteFileTool {
	w.ws = ws
	return w
}

// Name returns the tool name.
func (w *WriteFileTool) Name() string { return "write_text_file" }

// Description returns the tool description.
func (w *WriteFileTool) Description() string {
	return "Create, replace, or overwrite content in a text file. Supports line range replacement."
}

// Spec returns the JSON schema.
func (w *WriteFileTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        w.Name(),
		Description: w.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The target file path",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write",
				},
				"ranges": map[string]any{
					"type":        "string",
					"description": "Optional line range to replace, e.g., '[1,5]' or '1,5'",
				},
			},
			"required": []string{"file_path", "content"},
		},
	}
}

// Execute runs the tool.
func (w *WriteFileTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	filePath, _ := input["file_path"].(string)
	content, _ := input["content"].(string)
	ranges, _ := input["ranges"].(string)

	path, err := validatePath(filePath, w.baseDir)
	if err != nil {
		return nil, err
	}

	// Helper to check if file exists
	fileExists := func() bool {
		if w.ws != nil {
			_, err := w.ws.Stat(ctx, path)
			return err == nil
		}
		_, err := os.Stat(path)
		return err == nil
	}

	// Helper to read file
	readFile := func() ([]byte, error) {
		if w.ws != nil {
			return w.ws.ReadFile(ctx, path)
		}
		return os.ReadFile(path)
	}

	// Helper to write file
	writeFile := func(data []byte) error {
		if w.ws != nil {
			return w.ws.WriteFile(ctx, path, data, 0o644)
		}
		return os.WriteFile(path, data, 0o644)
	}

	// Helper to mkdir
	mkdirAll := func(dir string) error {
		if w.ws != nil {
			return w.ws.MkdirAll(ctx, dir, 0o755)
		}
		return os.MkdirAll(dir, 0o755)
	}

	// File doesn't exist: create it
	if !fileExists() {
		if err := mkdirAll(filepath.Dir(path)); err != nil {
			return nil, err
		}
		if err := writeFile([]byte(content)); err != nil {
			return nil, err
		}
		if strings.TrimSpace(ranges) != "" {
			return tool.NewTextResponse(fmt.Sprintf("Create and write %s successfully. The range %s was ignored because the file did not exist.", filePath, ranges)), nil
		}
		return tool.NewTextResponse(fmt.Sprintf("Create and write %s successfully.", filePath)), nil
	}

	data, err := readFile()
	if err != nil {
		return nil, err
	}
	originalLines := strings.Split(string(data), "\n")
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	if strings.TrimSpace(ranges) == "" {
		if err := writeFile([]byte(content)); err != nil {
			return nil, err
		}
		return tool.NewTextResponse(fmt.Sprintf("Overwrite %s successfully.", filePath)), nil
	}

	start, end, ok := parseRanges(ranges)
	if !ok {
		return nil, fmt.Errorf("invalid range format: expected '[start,end]' or 'start,end', got %s", ranges)
	}

	if start > len(originalLines) {
		return nil, fmt.Errorf("start line %d exceeds file length %d", start, len(originalLines))
	}

	var newLines []string
	if start > 1 {
		newLines = append(newLines, originalLines[:start-1]...)
	}
	newLines = append(newLines, strings.Split(content, "\n")...)
	if end < len(originalLines) {
		newLines = append(newLines, originalLines[end:]...)
	}

	if err := writeFile([]byte(strings.Join(newLines, "\n"))); err != nil {
		return nil, err
	}

	viewStart, viewEnd := calculateViewRanges(len(originalLines), len(newLines), start, end, 5)
	snippet := viewLines(newLines, viewStart, viewEnd)
	return tool.NewTextResponse(fmt.Sprintf("Write %s successfully. The new content snippet:\n```\n%s```", filePath, snippet)), nil
}

// InsertTextFileTool inserts content at a specific line.
type InsertTextFileTool struct {
	baseDir string
	ws      workspace.Workspace
}

// NewInsertTextFileTool creates a new InsertTextFileTool.
func NewInsertTextFileTool(baseDir string) *InsertTextFileTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &InsertTextFileTool{baseDir: baseDir}
}

// WithWorkspace binds the tool to a workspace for sandboxed execution.
func (i *InsertTextFileTool) WithWorkspace(ws workspace.Workspace) *InsertTextFileTool {
	i.ws = ws
	return i
}

// Name returns the tool name.
func (i *InsertTextFileTool) Name() string { return "insert_text_file" }

// Description returns the tool description.
func (i *InsertTextFileTool) Description() string {
	return "Insert content at a specific line number in a text file."
}

// Spec returns the JSON schema.
func (i *InsertTextFileTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        i.Name(),
		Description: i.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The target file path",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to insert",
				},
				"line_number": map[string]any{
					"type":        "number",
					"description": "Line number to insert at (1-based). Append if equals len+1.",
				},
			},
			"required": []string{"file_path", "content", "line_number"},
		},
	}
}

// Execute runs the tool.
func (i *InsertTextFileTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	filePath, _ := input["file_path"].(string)
	content, _ := input["content"].(string)
	lineNumberFloat, _ := input["line_number"].(float64)
	lineNumber := int(lineNumberFloat)

	if lineNumber <= 0 {
		return nil, fmt.Errorf("invalid line number: %d", lineNumber)
	}

	path, err := validatePath(filePath, i.baseDir)
	if err != nil {
		return nil, err
	}

	readFile := func() ([]byte, error) {
		if i.ws != nil {
			return i.ws.ReadFile(ctx, path)
		}
		return os.ReadFile(path)
	}
	writeFile := func(data []byte) error {
		if i.ws != nil {
			return i.ws.WriteFile(ctx, path, data, 0o644)
		}
		return os.WriteFile(path, data, 0o644)
	}
	fileExists := func() bool {
		if i.ws != nil {
			_, err := i.ws.Stat(ctx, path)
			return err == nil
		}
		_, err := os.Stat(path)
		return err == nil
	}

	if !fileExists() {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	data, err := readFile()
	if err != nil {
		return nil, err
	}
	originalLines := strings.Split(string(data), "\n")
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	if lineNumber > len(originalLines)+1 {
		return nil, fmt.Errorf("line number %d out of range [1, %d]", lineNumber, len(originalLines)+1)
	}

	var newLines []string
	if lineNumber <= len(originalLines) {
		newLines = append(newLines, originalLines[:lineNumber-1]...)
		newLines = append(newLines, strings.Split(content, "\n")...)
		newLines = append(newLines, originalLines[lineNumber-1:]...)
	} else {
		newLines = append(originalLines, strings.Split(content, "\n")...)
	}

	if err := writeFile([]byte(strings.Join(newLines, "\n"))); err != nil {
		return nil, err
	}

	viewStart, viewEnd := calculateViewRanges(len(originalLines), len(newLines), lineNumber, lineNumber, 5)
	snippet := viewLines(newLines, viewStart, viewEnd)
	return tool.NewTextResponse(fmt.Sprintf("Insert content into %s at line %d successfully. The content between lines %d-%d is:\n```\n%s```", filePath, lineNumber, viewStart, viewEnd, snippet)), nil
}
