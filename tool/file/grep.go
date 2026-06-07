package file

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/workspace"
)

// vcsDirs are directories to exclude from searches.
var vcsDirs = map[string]bool{
	".git": true, ".svn": true, ".hg": true, ".bzr": true,
}

// GrepTool searches file contents using regular expressions.
type GrepTool struct {
	baseDir string
	ws      workspace.Workspace
}

// NewGrepTool creates a new GrepTool with optional baseDir restriction.
func NewGrepTool(baseDir string) *GrepTool {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &GrepTool{baseDir: baseDir}
}

// WithWorkspace binds the tool to a workspace for sandboxed execution.
func (g *GrepTool) WithWorkspace(ws workspace.Workspace) *GrepTool {
	g.ws = ws
	return g
}

// Name returns the tool name.
func (g *GrepTool) Name() string { return "grep" }

// Description returns the tool description.
func (g *GrepTool) Description() string {
	return `A powerful search tool for finding text in files using regular expressions.

- Supports full regex syntax (e.g., "log.*Error", "func\s+\w+")
- Filter files with glob parameter (e.g., "*.go", "*.js")
- Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts per file
- Context lines: use context parameter for lines around matches
- Case-insensitive search: set i to true`
}

// Spec returns the JSON schema for the tool parameters.
func (g *GrepTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        g.Name(),
		Description: g.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "The regular expression pattern to search for in file contents.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory to search in. Defaults to current working directory.",
				},
				"output_mode": map[string]any{
					"type":        "string",
					"enum":        []string{"content", "files_with_matches", "count"},
					"description": "Output mode: 'content' shows matching lines, 'files_with_matches' shows file paths (default), 'count' shows match counts per file.",
				},
				"glob": map[string]any{
					"type":        "string",
					"description": "Glob pattern to filter files (e.g., '*.go', '*.js').",
				},
				"i": map[string]any{
					"type":        "boolean",
					"description": "Case insensitive search.",
				},
				"case_insensitive": map[string]any{
					"type":        "boolean",
					"description": "Case insensitive search (alias for i).",
				},
				"multiline": map[string]any{
					"type":        "boolean",
					"description": "Enable multiline mode where . matches newlines and patterns can span lines.",
				},
				"context": map[string]any{
					"type":        "integer",
					"description": "Number of context lines to show before and after matches. Requires output_mode: 'content'.",
				},
				"-A": map[string]any{
					"type":        "integer",
					"description": "Number of lines to show after each match. Requires output_mode: 'content'.",
				},
				"-B": map[string]any{
					"type":        "integer",
					"description": "Number of lines to show before each match. Requires output_mode: 'content'.",
				},
				"-C": map[string]any{
					"type":        "integer",
					"description": "Alias for context.",
				},
				"head_limit": map[string]any{
					"type":        "integer",
					"description": "Limit output to first N lines/entries. Defaults to 250. Pass 0 for unlimited.",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Skip first N lines/entries before applying head_limit.",
				},
				"n": map[string]any{
					"type":        "boolean",
					"description": "Show line numbers in output. Requires output_mode: 'content'. Defaults to true.",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

// matchRecord represents a single match.
type matchRecord struct {
	path    string
	lineNum int
	line    string
}

// Execute runs the grep search.
func (g *GrepTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	pattern, _ := input["pattern"].(string)
	searchPath, _ := input["path"].(string)
	outputMode, _ := input["output_mode"].(string)
	globPat, _ := input["glob"].(string)
	caseInsensitive, _ := input["i"].(bool)
	if !caseInsensitive {
		caseInsensitive, _ = input["case_insensitive"].(bool)
	}
	multiline, _ := input["multiline"].(bool)
	contextLines := intValue(input["context"])
	if contextLines == 0 {
		contextLines = intValue(input["-C"])
	}
	after := intValue(input["-A"])
	before := intValue(input["-B"])
	headLimit := intValue(input["head_limit"])
	if headLimit == 0 && input["head_limit"] == nil {
		headLimit = 250 // default
	}
	offset := intValue(input["offset"])
	showLineNum := true
	if v, ok := input["n"].(bool); ok {
		showLineNum = v
	}

	if strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	if searchPath == "" {
		searchPath = "."
	}
	if outputMode == "" {
		outputMode = "files_with_matches"
	}

	basePath, err := validatePath(searchPath, g.baseDir)
	if err != nil {
		return nil, err
	}

	// Compile regex
	flags := ""
	if caseInsensitive {
		flags += "(?i)"
	}
	if multiline {
		flags += "(?s)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Gather files to search
	files, err := g.collectFiles(ctx, basePath, globPat)
	if err != nil {
		return nil, err
	}

	// Search
	var matches []matchRecord
	fileCounts := make(map[string]int)
	filesWithMatches := make(map[string]bool)

	for _, f := range files {
		content, err := g.readFile(ctx, f)
		if err != nil {
			continue // skip unreadable files
		}

		if multiline {
			// Full text search
			if re.FindStringIndex(string(content)) != nil {
				filesWithMatches[f] = true
				fileCounts[f]++
				if outputMode == "content" {
					matches = append(matches, matchRecord{path: f, lineNum: 1, line: "[multiline match]"})
				}
			}
		} else {
			// Line-by-line search
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if re.MatchString(line) {
					filesWithMatches[f] = true
					fileCounts[f]++
					if outputMode == "content" {
						matches = append(matches, matchRecord{path: f, lineNum: lineNum, line: line})
					}
				}
			}
		}
	}

	if len(filesWithMatches) == 0 {
		return tool.NewTextResponse(fmt.Sprintf("No matches found for pattern: %s", pattern)), nil
	}

	switch outputMode {
	case "files_with_matches":
		var paths []string
		for p := range filesWithMatches {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		paths = applyPagination(paths, headLimit, offset)
		return tool.NewTextResponse(strings.Join(paths, "\n")), nil

	case "count":
		var items []string
		for p, c := range fileCounts {
			items = append(items, fmt.Sprintf("%s: %d", p, c))
		}
		sort.Strings(items)
		items = applyPagination(items, headLimit, offset)
		return tool.NewTextResponse(strings.Join(items, "\n")), nil

	case "content":
		return g.formatContent(matches, contextLines, before, after, showLineNum, headLimit, offset)

	default:
		return nil, fmt.Errorf("unknown output_mode: %s", outputMode)
	}
}

func (g *GrepTool) collectFiles(ctx context.Context, basePath, globPat string) ([]string, error) {
	info, err := g.stat(ctx, basePath)
	if err != nil {
		return nil, fmt.Errorf("path not found: %s", basePath)
	}

	// If basePath is a file, return it directly
	if !info.IsDir {
		if globPat != "" {
			matched, _ := filepath.Match(globPat, filepath.Base(basePath))
			if !matched {
				return nil, nil
			}
		}
		return []string{basePath}, nil
	}

	var files []string
	if g.ws != nil {
		g.walkWorkspace(ctx, basePath, globPat, &files)
	} else {
		_ = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if vcsDirs[filepath.Base(path)] {
					return filepath.SkipDir
				}
				return nil
			}
			if globPat != "" {
				matched, _ := filepath.Match(globPat, filepath.Base(path))
				if !matched {
					return nil
				}
			}
			files = append(files, path)
			return nil
		})
	}
	return files, nil
}

func (g *GrepTool) walkWorkspace(ctx context.Context, dir, globPat string, files *[]string) {
	entries, err := g.ws.ListDir(ctx, dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name)
		if e.IsDir {
			if vcsDirs[e.Name] {
				continue
			}
			g.walkWorkspace(ctx, fullPath, globPat, files)
		} else {
			if globPat != "" {
				matched, _ := filepath.Match(globPat, e.Name)
				if !matched {
					continue
				}
			}
			*files = append(*files, fullPath)
		}
	}
}

func (g *GrepTool) readFile(ctx context.Context, path string) ([]byte, error) {
	if g.ws != nil {
		return g.ws.ReadFile(ctx, path)
	}
	return os.ReadFile(path)
}

func (g *GrepTool) stat(ctx context.Context, path string) (workspace.FileInfo, error) {
	if g.ws != nil {
		return g.ws.Stat(ctx, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return workspace.FileInfo{}, err
	}
	return workspace.FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

func (g *GrepTool) formatContent(matches []matchRecord, contextLines, before, after int, showLineNum bool, headLimit, offset int) (*tool.Response, error) {
	if contextLines > 0 {
		before = contextLines
		after = contextLines
	}

	// Group matches by file for context handling
	fileMatches := make(map[string][]matchRecord)
	for _, m := range matches {
		fileMatches[m.path] = append(fileMatches[m.path], m)
	}

	var lines []string
	var filePaths []string
	for p := range fileMatches {
		filePaths = append(filePaths, p)
	}
	sort.Strings(filePaths)

	for _, path := range filePaths {
		ms := fileMatches[path]
		lines = append(lines, fmt.Sprintf("%s:", path))

		// Read full file to get context lines
		content, err := g.readFile(context.Background(), path)
		if err != nil {
			continue
		}
		fileLines := strings.Split(string(content), "\n")

		// Determine which lines to show
		shown := make(map[int]bool)
		for _, m := range ms {
			start := m.lineNum - before
			if start < 1 {
				start = 1
			}
			end := m.lineNum + after
			if end > len(fileLines) {
				end = len(fileLines)
			}
			for i := start; i <= end; i++ {
				shown[i] = true
			}
		}

		// Output lines in order with markers
		var prevLine int
		for i := 1; i <= len(fileLines); i++ {
			if !shown[i] {
				continue
			}
			if prevLine > 0 && i > prevLine+1 {
				lines = append(lines, "--")
			}
			isMatch := false
			for _, m := range ms {
				if m.lineNum == i {
					isMatch = true
					break
				}
			}
			marker := " "
			if isMatch {
				marker = ":"
			}
			if showLineNum {
				lines = append(lines, fmt.Sprintf("%d%s %s", i, marker, fileLines[i-1]))
			} else {
				lines = append(lines, fileLines[i-1])
			}
			prevLine = i
		}
	}

	lines = applyPagination(lines, headLimit, offset)

	suffix := ""
	if headLimit > 0 && len(lines) == headLimit {
		suffix = fmt.Sprintf("\n\n[Showing first %d lines]", headLimit)
	}

	return tool.NewTextResponse(strings.Join(lines, "\n") + suffix), nil
}

func applyPagination(items []string, limit, offset int) []string {
	if offset >= len(items) {
		return nil
	}
	items = items[offset:]
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}
