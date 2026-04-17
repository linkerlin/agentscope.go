package shell

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// ValidationResult is the outcome of command validation.
type ValidationResult struct {
	Allowed    bool
	Reason     string
	Executable string
}

// CommandValidator validates shell commands before execution.
type CommandValidator interface {
	Validate(command string, allowedCommands map[string]struct{}) *ValidationResult
	ExtractExecutable(command string) string
	ContainsMultipleCommands(command string) bool
}

func pathWithinCurrentDirectory(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, ".\\")
	segments := strings.Split(normalized, "/")
	depth := 0
	for _, seg := range segments {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			depth--
			if depth < 0 {
				return false
			}
		} else {
			depth++
		}
	}
	return depth >= 0
}

// defaultValidator returns a platform-specific validator.
func defaultValidator() CommandValidator {
	if runtime.GOOS == "windows" {
		return &WindowsCommandValidator{}
	}
	return &UnixCommandValidator{}
}

// UnixCommandValidator validates commands on Unix-like systems.
type UnixCommandValidator struct{}

func (u *UnixCommandValidator) Validate(command string, allowedCommands map[string]struct{}) *ValidationResult {
	executable := u.ExtractExecutable(command)
	if len(allowedCommands) == 0 {
		return &ValidationResult{Allowed: true, Executable: executable}
	}
	if u.ContainsMultipleCommands(command) {
		return &ValidationResult{Allowed: false, Reason: "Command contains multiple command separators (&, |, ;, newline)", Executable: executable}
	}
	if strings.HasPrefix(executable, "./") {
		if pathWithinCurrentDirectory(executable) {
			return &ValidationResult{Allowed: true, Executable: executable}
		}
		return &ValidationResult{Allowed: false, Reason: fmt.Sprintf("Command '%s' escapes current directory", executable), Executable: executable}
	}
	if _, ok := allowedCommands[executable]; ok {
		return &ValidationResult{Allowed: true, Executable: executable}
	}
	return &ValidationResult{Allowed: false, Reason: fmt.Sprintf("Command '%s' is not in the allowed whitelist", executable), Executable: executable}
}

func (u *UnixCommandValidator) ExtractExecutable(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	var executable string
	if strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, `'`) {
		quote := rune(trimmed[0])
		endQuote := strings.IndexRune(trimmed[1:], quote)
		if endQuote >= 0 {
			executable = trimmed[1 : endQuote+1]
		} else {
			executable = extractFirstToken(trimmed)
		}
	} else {
		executable = extractFirstToken(trimmed)
	}
	// filepath.Base also removes directory part
	base := filepath.Base(executable)
	for _, ext := range []string{".sh", ".py", ".rb", ".pl", ".bash", ".zsh"} {
		if strings.HasSuffix(base, ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	return base
}

func (u *UnixCommandValidator) ContainsMultipleCommands(command string) bool {
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	for _, c := range command {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if !inSingleQuote && !inDoubleQuote {
			if c == '&' || c == '|' || c == ';' || c == '\n' {
				return true
			}
		}
	}
	return false
}

// WindowsCommandValidator validates commands on Windows.
type WindowsCommandValidator struct{}

func (w *WindowsCommandValidator) Validate(command string, allowedCommands map[string]struct{}) *ValidationResult {
	executable := w.ExtractExecutable(command)
	if len(allowedCommands) == 0 {
		return &ValidationResult{Allowed: true, Executable: executable}
	}
	if w.ContainsMultipleCommands(command) {
		return &ValidationResult{Allowed: false, Reason: "Command contains multiple command separators (&, |, newline)", Executable: executable}
	}
	if strings.HasPrefix(executable, ".\\") || strings.HasPrefix(executable, "./") {
		if pathWithinCurrentDirectory(executable) {
			return &ValidationResult{Allowed: true, Executable: executable}
		}
		return &ValidationResult{Allowed: false, Reason: fmt.Sprintf("Command '%s' escapes current directory", executable), Executable: executable}
	}
	lowerExec := strings.ToLower(executable)
	for allowed := range allowedCommands {
		if strings.ToLower(allowed) == lowerExec {
			return &ValidationResult{Allowed: true, Executable: executable}
		}
	}
	return &ValidationResult{Allowed: false, Reason: fmt.Sprintf("Command '%s' is not in the allowed whitelist", executable), Executable: executable}
}

func (w *WindowsCommandValidator) ExtractExecutable(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	var executable string
	if strings.HasPrefix(trimmed, `"`) {
		endQuote := strings.Index(trimmed[1:], `"`)
		if endQuote >= 0 {
			executable = trimmed[1 : endQuote+1]
		} else {
			executable = extractFirstToken(trimmed)
		}
	} else if strings.ContainsAny(trimmed, "\\/") {
		executable = extractWindowsPath(trimmed)
	} else {
		executable = extractFirstToken(trimmed)
	}
	base := filepath.Base(executable)
	lowerBase := strings.ToLower(base)
	for _, ext := range []string{".exe", ".bat", ".cmd"} {
		if strings.HasSuffix(lowerBase, ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	return strings.ToLower(base)
}

func (w *WindowsCommandValidator) ContainsMultipleCommands(command string) bool {
	inDoubleQuote := false
	escaped := false
	for _, c := range command {
		if escaped {
			escaped = false
			continue
		}
		if c == '^' {
			escaped = true
			continue
		}
		if c == '"' {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if !inDoubleQuote {
			if c == '&' || c == '|' || c == '\n' {
				return true
			}
		}
	}
	return false
}

func extractFirstToken(command string) string {
	for i, c := range command {
		if unicode.IsSpace(c) {
			return command[:i]
		}
	}
	return command
}

func extractWindowsPath(command string) string {
	lower := strings.ToLower(command)
	best := -1
	for _, ext := range []string{".exe", ".bat", ".cmd"} {
		pos := strings.Index(lower, ext)
		if pos >= 0 && (best == -1 || pos < best) {
			best = pos
		}
	}
	if best >= 0 {
		end := best + 4
		if end >= len(command) || unicode.IsSpace(rune(command[end])) {
			return command[:end]
		}
	}
	return extractFirstToken(command)
}
