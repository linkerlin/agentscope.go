package shell

import (
	"strings"
	"unicode"

	"github.com/linkerlin/agentscope.go/tool"
)

func (s *ShellCommandTool) MatchRule(pattern string, input map[string]any) bool {
	command, _ := input["command"].(string)
	return tool.MatchBashCommand(pattern, command)
}

func (s *ShellCommandTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	command, _ := input["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	segments := splitCompoundCommand(command)
	if len(segments) == 0 {
		segments = []string{command}
	}

	seen := make(map[string]struct{})
	var rules []tool.SuggestedRule
	for _, segment := range segments {
		prefix := extractCommandPrefix(segment)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		rules = append(rules, tool.SuggestedRule{
			Name:     "suggested-bash-prefix",
			ToolName: s.Name(),
			Pattern:  prefix + ":*",
			Decision: tool.PermAllow,
		})
		if len(rules) >= 5 {
			break
		}
	}
	return rules
}

func splitCompoundCommand(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	var parts []string
	var cur strings.Builder
	inSingle, inDouble, inBacktick := false, false, false
	parenDepth := 0

	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			parts = append(parts, s)
		}
		cur.Reset()
	}

	atTop := func() bool { return !inSingle && !inDouble && !inBacktick && parenDepth == 0 }

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		if inSingle {
			cur.WriteByte(ch)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '\\' && i+1 < len(cmd) {
				cur.WriteByte(ch)
				cur.WriteByte(cmd[i+1])
				i++
				continue
			}
			cur.WriteByte(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			cur.WriteByte(ch)
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		switch ch {
		case '\'':
			inSingle = true
			cur.WriteByte(ch)
		case '"':
			inDouble = true
			cur.WriteByte(ch)
		case '`':
			inBacktick = true
			cur.WriteByte(ch)
		case '(':
			if i > 0 && cmd[i-1] == '$' {
				parenDepth++
			}
			cur.WriteByte(ch)
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			cur.WriteByte(ch)
		default:
			if atTop() && i+1 < len(cmd) {
				switch {
				case cmd[i:i+2] == "&&":
					flush()
					i++
					continue
				case cmd[i:i+2] == "||":
					flush()
					i++
					continue
				case ch == ';', ch == '|':
					flush()
					continue
				}
			}
			cur.WriteByte(ch)
		}
	}
	flush()
	return parts
}

func extractCommandPrefix(cmd string) string {
	tokens := shellTokenize(strings.TrimSpace(cmd))
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

func shellTokenize(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble, inBacktick := false, false, false
	parenDepth := 0
	escaped := false

	atTop := func() bool { return !inSingle && !inDouble && !inBacktick && parenDepth == 0 }

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
		if atTop() && ch == '\\' {
			escaped = true
			continue
		}
		if inSingle {
			cur.WriteByte(ch)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '\\' && i+1 < len(cmd) {
				cur.WriteByte(ch)
				cur.WriteByte(cmd[i+1])
				i++
				continue
			}
			cur.WriteByte(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			cur.WriteByte(ch)
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		switch ch {
		case '\'':
			inSingle = true
			cur.WriteByte(ch)
		case '"':
			inDouble = true
			cur.WriteByte(ch)
		case '`':
			inBacktick = true
			cur.WriteByte(ch)
		case '(':
			if i > 0 && cmd[i-1] == '$' {
				parenDepth++
			}
			cur.WriteByte(ch)
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			cur.WriteByte(ch)
		default:
			if atTop() && unicode.IsSpace(rune(ch)) {
				flush()
				continue
			}
			cur.WriteByte(ch)
		}
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
