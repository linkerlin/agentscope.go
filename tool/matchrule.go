package tool

import (
	"path/filepath"
	"regexp"
	"strings"
)

// MatchPathGlob matches a permission rule pattern against a file or directory path.
// Empty pattern matches all invocations (tool-name-level rule).
func MatchPathGlob(pattern, path string) bool {
	if pattern == "" {
		return true
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	path = filepath.ToSlash(path)
	matched, err := filepath.Match(pattern, path)
	return err == nil && matched
}

// SuggestPathParentGlob suggests a parent-directory glob for a file path.
func SuggestPathParentGlob(toolName, path string) []SuggestedRule {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parent := filepath.ToSlash(filepath.Dir(path))
	parent = strings.TrimSuffix(parent, "/")
	pattern := "**"
	if parent != "" && parent != "." {
		pattern = parent + "/**"
	}
	return []SuggestedRule{{
		Name:     "suggested-file-dir",
		ToolName: toolName,
		Pattern:  pattern,
		Decision: PermAllow,
	}}
}

// MatchGlobSearch matches a rule against glob tool path and/or pattern fields.
func MatchGlobSearch(rulePattern string, input map[string]any) bool {
	if rulePattern == "" {
		return true
	}
	if path, _ := input["path"].(string); path != "" && MatchPathGlob(rulePattern, path) {
		return true
	}
	if pattern, _ := input["pattern"].(string); pattern != "" && MatchPathGlob(rulePattern, pattern) {
		return true
	}
	return false
}

// SuggestGlobSearch suggests a directory glob for glob searches.
func SuggestGlobSearch(toolName, searchPath string) []SuggestedRule {
	searchPath = strings.TrimSpace(searchPath)
	if searchPath == "" {
		searchPath = "."
	}
	abs, err := filepath.Abs(searchPath)
	if err != nil {
		abs = searchPath
	}
	pattern := filepath.ToSlash(abs)
	pattern = strings.TrimSuffix(pattern, "/") + "/**"
	return []SuggestedRule{{
		Name:     "suggested-glob-dir",
		ToolName: toolName,
		Pattern:  pattern,
		Decision: PermAllow,
	}}
}

// MatchGrepPath matches a rule against the grep search path.
func MatchGrepPath(rulePattern, searchPath string) bool {
	if rulePattern == "" {
		return true
	}
	searchPath = strings.TrimSpace(searchPath)
	if searchPath == "" {
		if wd, err := filepath.Abs("."); err == nil {
			searchPath = wd
		}
	}
	return MatchPathGlob(rulePattern, searchPath)
}

// SuggestGrepPath suggests a directory glob for grep searches.
func SuggestGrepPath(toolName, searchPath string) []SuggestedRule {
	searchPath = strings.TrimSpace(searchPath)
	if searchPath == "" {
		searchPath = "."
	}
	abs, err := filepath.Abs(searchPath)
	if err != nil {
		abs = searchPath
	}
	pattern := filepath.ToSlash(abs)
	pattern = strings.TrimSuffix(pattern, "/") + "/**"
	return []SuggestedRule{{
		Name:     "suggested-grep-dir",
		ToolName: toolName,
		Pattern:  pattern,
		Decision: PermAllow,
	}}
}

// MatchBashCommand matches Bash permission rules against a command string (PyV2 Bash.match_rule).
func MatchBashCommand(rulePattern, command string) bool {
	if rulePattern == "" {
		return true
	}
	command = strings.TrimSpace(command)

	if strings.HasSuffix(rulePattern, ":*") {
		prefix := strings.TrimSpace(rulePattern[:len(rulePattern)-2])
		return strings.HasPrefix(command, prefix+" ") || command == prefix
	}

	if !bashPatternHasWildcards(rulePattern) {
		pattern := unescapeBashPattern(rulePattern)
		return strings.Contains(command, pattern)
	}

	regexPattern := bashWildcardToRegex(rulePattern)
	if strings.HasSuffix(regexPattern, ".*") {
		base := strings.TrimRight(regexPattern[:len(regexPattern)-2], " ")
		if matched, _ := regexp.MatchString("^"+base+"$", command); matched {
			return true
		}
	}
	matched, err := regexp.MatchString("^"+regexPattern+"$", command)
	if err != nil {
		return strings.Contains(command, strings.ReplaceAll(rulePattern, "*", ""))
	}
	return matched
}

func bashPatternHasWildcards(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' {
			i++
			continue
		}
		if pattern[i] == '*' {
			return true
		}
	}
	return false
}

func unescapeBashPattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "\\\\", "\x00BACKSLASH\x00")
	pattern = strings.ReplaceAll(pattern, "\\*", "*")
	pattern = strings.ReplaceAll(pattern, "\x00BACKSLASH\x00", "\\")
	return pattern
}

func bashWildcardToRegex(pattern string) string {
	const escapedStar = "\x00ESCAPED_STAR\x00"
	const escapedBackslash = "\x00ESCAPED_BACKSLASH\x00"

	pattern = strings.ReplaceAll(pattern, "\\\\", escapedBackslash)
	pattern = strings.ReplaceAll(pattern, "\\*", escapedStar)

	for _, ch := range []string{".", "^", "$", "+", "?", "{", "}", "[", "]", "|", "(", ")"} {
		pattern = strings.ReplaceAll(pattern, ch, "\\"+ch)
	}
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = strings.ReplaceAll(pattern, escapedStar, `\*`)
	pattern = strings.ReplaceAll(pattern, escapedBackslash, `\\`)
	return pattern
}
