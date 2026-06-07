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
	"github.com/linkerlin/agentscope.go/workspace"
)

// GlobTool provides fast file pattern matching.
type GlobTool struct {
	baseDir string
	ws      workspace.Workspace
}

// NewGlobTool creates a new GlobTool with optional baseDir restriction.
func NewGlobTool(baseDir string) *GlobTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &GlobTool{baseDir: baseDir}
}

// WithWorkspace binds the tool to a workspace for sandboxed execution.
func (g *GlobTool) WithWorkspace(ws workspace.Workspace) *GlobTool {
	g.ws = ws
	return g
}

// Name returns the tool name.
func (g *GlobTool) Name() string { return "glob" }

// Description returns the tool description.
func (g *GlobTool) Description() string {
	return `Fast file pattern matching tool.

Supports glob patterns like "**/*.go" or "src/**/*.ts" and returns matching file paths sorted by modification time (newest first).`
}

// Spec returns the JSON schema for the tool parameters.
func (g *GlobTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        g.Name(),
		Description: g.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "The glob pattern to match against (e.g., '**/*.py', 'src/**/*.ts')",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "The base directory to search from (defaults to current working directory)",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

// Execute runs the glob pattern matching.
func (g *GlobTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	pattern, _ := input["pattern"].(string)
	searchPath, _ := input["path"].(string)

	if strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	if searchPath == "" {
		searchPath = "."
	}

	basePath, err := validatePath(searchPath, g.baseDir)
	if err != nil {
		return nil, err
	}

	var matches []string
	if g.ws != nil {
		matches, err = g.globWorkspace(ctx, pattern, basePath)
	} else {
		matches, err = g.globFS(pattern, basePath)
	}
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return tool.NewTextResponse(fmt.Sprintf("No files found matching pattern: %s", pattern)), nil
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		var ti, tj int64
		if g.ws != nil {
			fi, err1 := g.ws.Stat(ctx, matches[i])
			fj, err2 := g.ws.Stat(ctx, matches[j])
			if err1 == nil {
				ti = fi.ModTime.Unix()
			}
			if err2 == nil {
				tj = fj.ModTime.Unix()
			}
		} else {
			fi, _ := os.Stat(matches[i])
			fj, _ := os.Stat(matches[j])
			if fi != nil {
				ti = fi.ModTime().Unix()
			}
			if fj != nil {
				tj = fj.ModTime().Unix()
			}
		}
		return ti > tj
	})

	return tool.NewTextResponse(strings.Join(matches, "\n")), nil
}

func (g *GlobTool) globFS(pattern, basePath string) ([]string, error) {
	parts := splitPattern(pattern)
	var results []string
	g.matchParts(parts, 0, basePath, &results, false)
	return results, nil
}

func (g *GlobTool) globWorkspace(ctx context.Context, pattern, basePath string) ([]string, error) {
	parts := splitPattern(pattern)
	var results []string
	g.matchPartsWorkspace(ctx, parts, 0, basePath, &results)
	return results, nil
}

func splitPattern(pattern string) []string {
	pattern = filepath.Clean(pattern)
	parts := strings.Split(pattern, string(filepath.Separator))
	// Handle leading separator from absolute paths
	if len(parts) > 0 && parts[0] == "." {
		parts = parts[1:]
	}
	return parts
}

func (g *GlobTool) matchParts(parts []string, idx int, currentDir string, results *[]string, isAbs bool) {
	if idx >= len(parts) {
		return
	}

	part := parts[idx]
	isLast := idx == len(parts)-1

	if part == "**" {
		if isLast {
			g.collectAll(currentDir, results)
		} else {
			// Match in current directory (skip ** and continue)
			g.matchParts(parts, idx+1, currentDir, results, isAbs)
			// Recursively match in subdirectories
			entries, _ := os.ReadDir(currentDir)
			for _, e := range entries {
				if e.IsDir() {
					g.matchParts(parts, idx, filepath.Join(currentDir, e.Name()), results, isAbs)
				}
			}
		}
		return
	}

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		matched, _ := filepath.Match(part, e.Name())
		if !matched {
			continue
		}
		fullPath := filepath.Join(currentDir, e.Name())
		if isLast {
			if !e.IsDir() {
				*results = append(*results, fullPath)
			}
		} else if e.IsDir() {
			g.matchParts(parts, idx+1, fullPath, results, isAbs)
		}
	}
}

func (g *GlobTool) matchPartsWorkspace(ctx context.Context, parts []string, idx int, currentDir string, results *[]string) {
	if idx >= len(parts) {
		return
	}

	part := parts[idx]
	isLast := idx == len(parts)-1

	if part == "**" {
		if isLast {
			g.collectAllWorkspace(ctx, currentDir, results)
		} else {
			g.matchPartsWorkspace(ctx, parts, idx+1, currentDir, results)
			entries, _ := g.ws.ListDir(ctx, currentDir)
			for _, e := range entries {
				if e.IsDir {
					g.matchPartsWorkspace(ctx, parts, idx, filepath.Join(currentDir, e.Name), results)
				}
			}
		}
		return
	}

	entries, err := g.ws.ListDir(ctx, currentDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		matched, _ := filepath.Match(part, e.Name)
		if !matched {
			continue
		}
		fullPath := filepath.Join(currentDir, e.Name)
		if isLast {
			if !e.IsDir {
				*results = append(*results, fullPath)
			}
		} else if e.IsDir {
			g.matchPartsWorkspace(ctx, parts, idx+1, fullPath, results)
		}
	}
}

func (g *GlobTool) collectAll(currentDir string, results *[]string) {
	_ = filepath.WalkDir(currentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			*results = append(*results, path)
		}
		return nil
	})
}

func (g *GlobTool) collectAllWorkspace(ctx context.Context, currentDir string, results *[]string) {
	entries, err := g.ws.ListDir(ctx, currentDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		fullPath := filepath.Join(currentDir, e.Name)
		if e.IsDir {
			g.collectAllWorkspace(ctx, fullPath, results)
		} else {
			*results = append(*results, fullPath)
		}
	}
}
