package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// ReadFileTool provides file viewing and directory listing capabilities.
type ReadFileTool struct {
	baseDir string
}

// NewReadFileTool creates a new ReadFileTool with optional baseDir restriction.
func NewReadFileTool(baseDir string) *ReadFileTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &ReadFileTool{baseDir: baseDir}
}

// Name returns the tool name.
func (r *ReadFileTool) Name() string { return "view_text_file" }

// Description returns the tool description.
func (r *ReadFileTool) Description() string {
	return "View file content in specified range with line numbers, or list directory contents."
}

// Spec returns the JSON schema for the tool parameters.
func (r *ReadFileTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        r.Name(),
		Description: r.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The target file path",
				},
				"ranges": map[string]any{
					"type":        "string",
					"description": "Line range to view, e.g., '1,100' or '[1,100]'. Use '-100,-1' for last 100 lines.",
				},
			},
			"required": []string{"file_path"},
		},
	}
}

// Execute runs the tool.
func (r *ReadFileTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	filePath, _ := input["file_path"].(string)
	ranges, _ := input["ranges"].(string)

	path, err := validatePath(filePath, r.baseDir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, use list_directory tool: %s", filePath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if strings.TrimSpace(ranges) == "" {
		content, err := viewTextFile(path, 1, len(lines))
		if err != nil {
			return nil, err
		}
		return tool.NewTextResponse(fmt.Sprintf("The content of %s:\n```\n%s```", filePath, content)), nil
	}

	start, end, ok := parseRanges(ranges)
	if !ok {
		return nil, fmt.Errorf("invalid range format: expected '[start,end]' or 'start,end', got %s", ranges)
	}

	// Handle negative indices
	if start < 0 {
		start = len(lines) + start + 1
	}
	if end < 0 {
		end = len(lines) + end + 1
	}
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return nil, fmt.Errorf("invalid range: start line %d is greater than end line %d", start, end)
	}

	content, err := viewTextFile(path, start, end)
	if err != nil {
		return nil, err
	}
	return tool.NewTextResponse(fmt.Sprintf("The content of %s in lines [%d, %d]:\n```\n%s```", filePath, start, end, content)), nil
}

// ListDirectoryTool lists files and directories.
type ListDirectoryTool struct {
	baseDir string
}

// NewListDirectoryTool creates a new ListDirectoryTool.
func NewListDirectoryTool(baseDir string) *ListDirectoryTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &ListDirectoryTool{baseDir: baseDir}
}

// Name returns the tool name.
func (l *ListDirectoryTool) Name() string { return "list_directory" }

// Description returns the tool description.
func (l *ListDirectoryTool) Description() string {
	return "List all files and directories in the specified directory."
}

// Spec returns the JSON schema.
func (l *ListDirectoryTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        l.Name(),
		Description: l.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dir_path": map[string]any{
					"type":        "string",
					"description": "The target directory path",
				},
			},
			"required": []string{"dir_path"},
		},
	}
}

// Execute runs the tool.
func (l *ListDirectoryTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	dirPath, _ := input["dir_path"].(string)
	path, err := validatePath(dirPath, l.baseDir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("directory does not exist: %s", dirPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", dirPath)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var dirs, files []string
	for _, e := range entries {
		full := filepath.Join(path, e.Name())
		if e.IsDir() {
			dirs = append(dirs, full)
		} else {
			files = append(files, full)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)

	var sb strings.Builder
	fmt.Fprintf(&sb, "Contents of directory %s:\n\n", dirPath)
	if len(dirs) > 0 {
		sb.WriteString("Directories:\n")
		for _, d := range dirs {
			fmt.Fprintf(&sb, "  %s\n", d)
		}
		sb.WriteString("\n")
	}
	if len(files) > 0 {
		sb.WriteString("Files:\n")
		for _, f := range files {
			fmt.Fprintf(&sb, "  %s\n", f)
		}
		sb.WriteString("\n")
	}
	if len(dirs) == 0 && len(files) == 0 {
		sb.WriteString("(empty directory)\n")
	} else {
		fmt.Fprintf(&sb, "Total: %d director(y/ies), %d file(s)", len(dirs), len(files))
	}

	return tool.NewTextResponse(sb.String()), nil
}
