package permission

import (
	pathpkg "path"
	"path/filepath"
	"strings"
)

// IsDangerousPath checks if a file path contains a sensitive file or directory.
// Matching is case-insensitive on Windows and macOS.
func IsDangerousPath(path string, files, dirs []string) bool {
	if path == "" {
		return false
	}

	// Normalize path separators
	path = filepath.Clean(path)
	base := filepath.Base(path)

	// Check basename against dangerous files
	for _, f := range files {
		if stringsEqualFold(base, f) {
			return true
		}
	}

	// Check path segments against dangerous directories
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		for _, d := range dirs {
			if stringsEqualFold(part, d) {
				return true
			}
		}
	}

	return false
}

// IsDangerousCommand checks if a command matches any dangerous pattern,
// including subshell bodies in compound commands.
func IsDangerousCommand(cmd string) bool {
	return AnyDangerousCommand(cmd)
}

// IsReadOnlyCommand checks if a command is a known safe read-only operation.
func IsReadOnlyCommand(cmd string) bool {
	return AllReadOnlyCommands(cmd)
}

// IsDangerousRemoval checks if an rm/rmdir command targets a critical system path.
func IsDangerousRemoval(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	tokens := strings.Fields(cmd)
	if len(tokens) == 0 {
		return false
	}
	base := tokens[0]
	if base != "rm" && base != "rmdir" {
		return false
	}

	// Collect non-flag arguments as potential paths
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		if strings.HasPrefix(tok, "-") {
			continue
		}
		path := strings.Trim(tok, `"'`)
		if isCriticalSystemPath(path) {
			return true
		}
	}
	return false
}

// isCriticalSystemPath checks if a path is a critical system directory that
// must not be removed.
func isCriticalSystemPath(path string) bool {
	if path == "" {
		return false
	}
	// Bare wildcard or root
	if path == "*" || path == "./*" || path == "/" || path == "~" {
		return true
	}
	// Ends with /* — removes everything in a directory
	if strings.HasSuffix(path, "/*") || strings.HasSuffix(path, `\*`) {
		return true
	}

	// Expand home
	if strings.HasPrefix(path, "~") {
		return true
	}

	// Normalize using Unix-style path (commands target Linux sandboxes).
	abs := pathpkg.Clean(path)
	if abs == "/" || abs == "." || abs == ".." {
		return true
	}

	// Direct children of root on Unix
	dir := pathpkg.Dir(abs)
	if dir == "/" && abs != "/" {
		return true
	}

	return false
}

// stringsEqualFold is a helper for case-insensitive string comparison.
func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(a, b)
}
