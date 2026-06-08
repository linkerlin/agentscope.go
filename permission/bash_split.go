package permission

import "strings"

// ParserBackend reports which bash split implementation is active.
func ParserBackend() string {
	return parserBackendName()
}

// SplitCompoundCommand splits shell commands into permission-checkable segments.
// With -tags treesitter and CGO enabled, tree-sitter-bash is used; otherwise heuristic.
func SplitCompoundCommand(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	parts := splitCompoundCommandImpl(cmd)
	if len(parts) == 0 {
		return []string{cmd}
	}
	return parts
}
