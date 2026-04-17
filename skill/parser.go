package skill

import (
	"fmt"
	"regexp"
	"strings"
)

// ParsedMarkdown 保存从 Markdown 中提取的 YAML frontmatter 和正文
type ParsedMarkdown struct {
	Metadata map[string]string
	Content  string
}

var frontmatterPattern = regexp.MustCompile(`(?s)^---\s*[\r\n]+(.*?)[\r\n]*---(?:\s*[\r\n]+)?(.*)`)

// ParseMarkdownWithFrontmatter 解析带有 YAML frontmatter 的 Markdown
func ParseMarkdownWithFrontmatter(markdown string) (*ParsedMarkdown, error) {
	if strings.TrimSpace(markdown) == "" {
		return &ParsedMarkdown{Metadata: map[string]string{}, Content: ""}, nil
	}
	matches := frontmatterPattern.FindStringSubmatch(markdown)
	if len(matches) == 0 {
		return &ParsedMarkdown{Metadata: map[string]string{}, Content: markdown}, nil
	}
	yamlContent := strings.TrimSpace(matches[1])
	content := matches[2]
	if yamlContent == "" {
		return &ParsedMarkdown{Metadata: map[string]string{}, Content: content}, nil
	}
	metadata := parseSimpleYAML(yamlContent)
	return &ParsedMarkdown{Metadata: metadata, Content: content}, nil
}

func parseSimpleYAML(yaml string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		rawValue := strings.TrimSpace(parts[1])
		if rawValue == "|" || rawValue == ">" {
			// 跳过块标量修饰符（简单解析器不支持）
			continue
		}
		result[key] = parseYAMLValue(rawValue)
	}
	return result
}

func parseYAMLValue(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return unescapeYAMLString(v[1 : len(v)-1])
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	return v
}

func unescapeYAMLString(s string) string {
	var sb strings.Builder
	escaped := false
	for _, c := range s {
		if escaped {
			switch c {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte('\\')
				sb.WriteRune(c)
			}
			escaped = false
		} else if c == '\\' {
			escaped = true
		} else {
			sb.WriteRune(c)
		}
	}
	if escaped {
		sb.WriteByte('\\')
	}
	return sb.String()
}

// GenerateMarkdownWithFrontmatter 根据 metadata 和 content 生成 Markdown
func GenerateMarkdownWithFrontmatter(metadata map[string]string, content string) string {
	var sb strings.Builder
	if len(metadata) > 0 {
		sb.WriteString("---\n")
		for k, v := range metadata {
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, quoteYAMLValueIfNeeded(v)))
		}
		sb.WriteString("---\n")
		if content != "" {
			sb.WriteString("\n")
		}
	}
	sb.WriteString(content)
	return sb.String()
}

func quoteYAMLValueIfNeeded(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, ":#\n\r\t") || strings.HasPrefix(v, " ") || strings.HasSuffix(v, " ") {
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `"`, `\"`)
		v = strings.ReplaceAll(v, "\n", `\n`)
		v = strings.ReplaceAll(v, "\r", `\r`)
		v = strings.ReplaceAll(v, "\t", `\t`)
		return `"` + v + `"`
	}
	first := v[0]
	if first == '"' || first == '\'' || first == '[' || first == ']' || first == '{' || first == '}' || first == '>' || first == '|' || first == '*' || first == '&' || first == '!' || first == '%' || first == '@' || first == '`' {
		return `"` + v + `"`
	}
	return v
}
