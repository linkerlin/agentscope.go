package permission

import (
	"strings"
)

// SplitCompoundCommand splits shell commands on &&, ||, ; and | while
// preserving simple quoted segments. This is a lightweight alternative to
// PyV2's tree-sitter bash parser for permission evaluation.
func SplitCompoundCommand(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	var parts []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			cur.WriteByte(ch)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			cur.WriteByte(ch)
		default:
			if !inSingle && !inDouble {
				if ch == ';' {
					if s := strings.TrimSpace(cur.String()); s != "" {
						parts = append(parts, s)
					}
					cur.Reset()
					continue
				}
				if i+1 < len(cmd) && cmd[i] == '&' && cmd[i+1] == '&' {
					if s := strings.TrimSpace(cur.String()); s != "" {
						parts = append(parts, s)
					}
					cur.Reset()
					i++
					continue
				}
				if i+1 < len(cmd) && cmd[i] == '|' && cmd[i+1] == '|' {
					if s := strings.TrimSpace(cur.String()); s != "" {
						parts = append(parts, s)
					}
					cur.Reset()
					i++
					continue
				}
				if ch == '|' {
					if s := strings.TrimSpace(cur.String()); s != "" {
						parts = append(parts, s)
					}
					cur.Reset()
					continue
				}
			}
			cur.WriteByte(ch)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return []string{cmd}
	}
	return parts
}

// AnyDangerousCommand returns true if any sub-command in a compound command is dangerous.
func AnyDangerousCommand(cmd string) bool {
	for _, part := range SplitCompoundCommand(cmd) {
		if IsDangerousCommand(part) || IsDangerousRemoval(part) {
			return true
		}
	}
	return false
}

// AllReadOnlyCommands returns true only if every sub-command is read-only.
func AllReadOnlyCommands(cmd string) bool {
	parts := SplitCompoundCommand(cmd)
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if !IsReadOnlyCommand(part) {
			return false
		}
	}
	return true
}
