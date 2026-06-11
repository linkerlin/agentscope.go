package permission

import (
	"strings"
	"unicode"
)

type shellScanState struct {
	inSingle   bool
	inDouble   bool
	inBacktick bool
	parenDepth int
}

func (s *shellScanState) atTopLevel() bool {
	return !s.inSingle && !s.inDouble && !s.inBacktick && s.parenDepth == 0
}

// advance processes one rune and updates quote/paren depth.
func (s *shellScanState) advance(cmd string, i int) {
	if s.inSingle {
		if cmd[i] == '\'' {
			s.inSingle = false
		}
		return
	}
	if s.inDouble {
		if cmd[i] == '\\' && i+1 < len(cmd) {
			return
		}
		if cmd[i] == '"' {
			s.inDouble = false
		}
		return
	}
	if s.inBacktick {
		if cmd[i] == '`' {
			s.inBacktick = false
		}
		return
	}

	switch cmd[i] {
	case '\'':
		s.inSingle = true
	case '"':
		s.inDouble = true
	case '`':
		s.inBacktick = true
	case '(':
		if i > 0 && cmd[i-1] == '$' {
			s.parenDepth++
		}
	case ')':
		if s.parenDepth > 0 {
			s.parenDepth--
		}
	}
}

// splitCompoundCommandHeuristic splits shell commands on &&, ||, ; and | while
// respecting quotes and $(...) subshells.
func splitCompoundCommandHeuristic(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	var parts []string
	var cur strings.Builder
	var st shellScanState

	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			parts = append(parts, s)
		}
		cur.Reset()
	}

	for i := 0; i < len(cmd); i++ {
		st.advance(cmd, i)
		if !st.atTopLevel() {
			cur.WriteByte(cmd[i])
			continue
		}

		switch cmd[i] {
		case ';':
			flush()
			continue
		case '&':
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				flush()
				i++
				continue
			}
		case '|':
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				flush()
				i++
				continue
			}
			flush()
			continue
		}
		cur.WriteByte(cmd[i])
	}
	flush()

	if len(parts) == 0 {
		return []string{cmd}
	}
	return parts
}

// ExtractSubshellBodies returns inner command strings from $(...) and `...`.
func ExtractSubshellBodies(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	var out []string
	var st shellScanState
	for i := 0; i < len(cmd); i++ {
		prev := st
		st.advance(cmd, i)

		if !prev.inBacktick && st.inBacktick {
			if body := readUntil(cmd, &i, '`'); body != "" {
				out = append(out, body)
			}
			continue
		}
		if prev.parenDepth == 0 && st.parenDepth == 1 && i > 0 && cmd[i] == '(' && cmd[i-1] == '$' {
			if body := readUntil(cmd, &i, ')'); body != "" {
				out = append(out, body)
			}
			st.parenDepth = 0
		}
	}
	return out
}

func readUntil(cmd string, i *int, end byte) string {
	start := *i + 1
	depth := 1
	var st shellScanState
	for j := start; j < len(cmd); j++ {
		if end == ')' {
			st.advance(cmd, j)
			if st.atTopLevel() && cmd[j] == ')' {
				depth--
				if depth == 0 {
					*i = j
					return cmd[start:j]
				}
			}
			continue
		}
		if cmd[j] == end && st.atTopLevel() {
			*i = j
			return cmd[start:j]
		}
		st.advance(cmd, j)
	}
	return ""
}

// HasOutputRedirection reports whether cmd redirects stdout/stderr outside quotes.
func HasOutputRedirection(cmd string) bool {
	var st shellScanState
	for i := 0; i < len(cmd); i++ {
		st.advance(cmd, i)
		if !st.atTopLevel() {
			continue
		}
		if cmd[i] == '>' {
			if i > 0 && cmd[i-1] == '2' {
				continue // 2>&1
			}
			return true
		}
	}
	return false
}

// ShellTokenize splits a simple command into words respecting quotes.
func ShellTokenize(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	var tokens []string
	var cur strings.Builder
	var st shellScanState
	escaped := false

	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			tokens = append(tokens, unquoteToken(s))
		}
		cur.Reset()
	}

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		if escaped {
			cur.WriteByte(ch)
			escaped = false
			continue
		}
		if !st.inSingle && !st.inDouble && !st.inBacktick && ch == '\\' {
			escaped = true
			continue
		}

		st.advance(cmd, i)
		if st.atTopLevel() && unicode.IsSpace(rune(ch)) {
			flush()
			continue
		}
		cur.WriteByte(ch)
	}
	flush()
	return tokens
}

func unquoteToken(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return strings.Trim(s, `"'`)
}

// ExtractCommandPrefix returns the first two command words (e.g. "npm run").
func ExtractCommandPrefix(cmd string) string {
	tokens := ShellTokenize(strings.TrimSpace(cmd))
	if len(tokens) == 0 {
		return ""
	}
	for len(tokens) > 0 && isEnvAssignment(tokens[0]) {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) == 1 {
		return tokens[0]
	}
	if strings.HasPrefix(tokens[1], "-") {
		return tokens[0]
	}
	return tokens[0] + " " + tokens[1]
}

func isEnvAssignment(tok string) bool {
	i := strings.IndexByte(tok, '=')
	if i <= 0 {
		return false
	}
	name := tok[:i]
	for _, ch := range name {
		if !(unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_') {
			return false
		}
	}
	return true
}

// ExtractFilePaths heuristically extracts path-like arguments from a command.
func ExtractFilePaths(cmd string) []string {
	tokens := ShellTokenize(strings.TrimSpace(cmd))
	if len(tokens) == 0 {
		return nil
	}
	pathCommands := map[string]bool{
		"rm": true, "rmdir": true, "cp": true, "mv": true, "touch": true,
		"mkdir": true, "chmod": true, "chown": true, "cat": true, "head": true, "tail": true,
	}
	if !pathCommands[strings.ToLower(tokens[0])] {
		return nil
	}
	var paths []string
	for _, tok := range tokens[1:] {
		if strings.HasPrefix(tok, "-") {
			continue
		}
		if tok == ">" || tok == ">>" || tok == "|" {
			break
		}
		paths = append(paths, tok)
	}
	return paths
}

// AnyDangerousCommand returns true if any sub-command or subshell is dangerous.
func AnyDangerousCommand(cmd string) bool {
	return walkSafetySegments(cmd, func(part string) bool {
		return segmentDangerous(part)
	})
}

// AllReadOnlyCommands returns true only if every sub-command is read-only.
func AllReadOnlyCommands(cmd string) bool {
	if HasOutputRedirection(cmd) {
		return false
	}
	parts := SplitCompoundCommand(cmd)
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if !isSegmentReadOnly(part) {
			return false
		}
	}
	return true
}

func walkSafetySegments(cmd string, pred func(string) bool) bool {
	for _, part := range SplitCompoundCommand(cmd) {
		if pred(part) {
			return true
		}
		for _, sub := range ExtractSubshellBodies(part) {
			if walkSafetySegments(sub, pred) {
				return true
			}
		}
	}
	return false
}

func segmentDangerous(part string) bool {
	part = strings.TrimSpace(part)
	if IsDangerousCommandSingle(part) || IsDangerousRemoval(part) {
		return true
	}
	return false
}

func isSegmentReadOnly(part string) bool {
	part = strings.TrimSpace(part)
	if HasOutputRedirection(part) {
		return false
	}
	for _, sub := range ExtractSubshellBodies(part) {
		if !AllReadOnlyCommands(sub) {
			return false
		}
	}
	return IsReadOnlyCommandSingle(part)
}

// IsDangerousCommandSingle checks a single non-compound segment.
func IsDangerousCommandSingle(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	for _, pattern := range DefaultDangerousCommands {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}
	return false
}

// IsReadOnlyCommandSingle checks a single non-compound segment.
func IsReadOnlyCommandSingle(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	for _, safe := range DefaultReadOnlyCommands {
		if strings.HasPrefix(cmd, safe) {
			rest := strings.TrimPrefix(cmd, safe)
			if rest == "" || strings.HasPrefix(rest, " ") {
				return true
			}
		}
	}
	prefix := ExtractCommandPrefix(cmd)
	for _, safe := range DefaultReadOnlyCommands {
		if prefix == safe || strings.HasPrefix(prefix, safe+" ") {
			return true
		}
	}
	return false
}
