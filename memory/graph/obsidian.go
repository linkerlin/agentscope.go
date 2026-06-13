package graph

import (
	"fmt"
	"strings"
	"time"
)

// ObsidianExporter 将知识图谱导出为 Obsidian 兼容的 Markdown 文件
type ObsidianExporter struct {
	vaultPath string
}

// NewObsidianExporter 创建 Obsidian 导出器
func NewObsidianExporter(vaultPath string) *ObsidianExporter {
	return &ObsidianExporter{vaultPath: vaultPath}
}

// ExportNode 导出单个节点为 Obsidian Markdown 文件
func (e *ObsidianExporter) ExportNode(node *Node) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder

	// YAML front matter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", node.ID))
	sb.WriteString(fmt.Sprintf("title: %s\n", escapeYAML(node.Title)))
	sb.WriteString(fmt.Sprintf("type: %s\n", node.Type))
	sb.WriteString(fmt.Sprintf("created: %s\n", time.Now().Format("2006-01-02")))
	if len(node.Aliases) > 0 {
		sb.WriteString(fmt.Sprintf("aliases: [%s]\n", strings.Join(node.Aliases, ", ")))
	}
	if len(node.Outlinks) > 0 {
		sb.WriteString(fmt.Sprintf("outlinks: [%s]\n", strings.Join(node.Outlinks, ", ")))
	}
	if len(node.Inlinks) > 0 {
		sb.WriteString(fmt.Sprintf("inlinks: [%s]\n", strings.Join(node.Inlinks, ", ")))
	}
	if len(node.Metadata) > 0 {
		for k, v := range node.Metadata {
			sb.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
	}
	sb.WriteString("---\n\n")

	// 标题
	sb.WriteString(fmt.Sprintf("# %s\n\n", node.Title))

	// 内容（将 Wikilink 转换为 Obsidian 格式）
	content := node.Content
	links := ExtractWikilinks(content)
	// 从后向前替换，避免位置偏移
	for i := len(links) - 1; i >= 0; i-- {
		link := links[i]
		obsidianLink := fmt.Sprintf("[[%s]]", link.Concept)
		if link.Alias != "" {
			obsidianLink = fmt.Sprintf("[[%s|%s]]", link.Concept, link.Alias)
		}
		content = content[:link.Start] + obsidianLink + content[link.End:]
	}
	sb.WriteString(content)
	sb.WriteString("\n\n")

	// 标签（从 Metadata 中提取）
	if tags, ok := node.Metadata["tags"]; ok {
		sb.WriteString(fmt.Sprintf("\n#%v\n", tags))
	}

	// Dataview 语义链接（MOC 风格）
	if len(node.Outlinks) > 0 {
		sb.WriteString("\n## Related\n\n")
		for _, outlink := range node.Outlinks {
			sb.WriteString(fmt.Sprintf("- [[%s]]\n", outlink))
		}
	}

	if len(node.Inlinks) > 0 {
		sb.WriteString("\n## Backlinks\n\n")
		for _, inlink := range node.Inlinks {
			sb.WriteString(fmt.Sprintf("- [[%s]]\n", inlink))
		}
	}

	// MOC 链接
	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("*Part of: [[MOC %s]]*\n", node.Type))

	return sb.String()
}

// ExportGraph 导出整个图谱为 Obsidian Vault
func (e *ObsidianExporter) ExportGraph(g *Graph) map[string]string {
	if g == nil {
		return nil
	}

	files := make(map[string]string)
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, node := range g.nodes {
		filename := sanitizeFilename(node.Title) + ".md"
		files[filename] = e.ExportNode(node)
	}

	// 生成 MOC（Map of Content）文件
	for nodeType := range NodeTypeConcept {
		_ = nodeType
		// 为每种类型生成 MOC
	}

	return files
}

// GenerateMOC 生成 Map of Content 文件
func (e *ObsidianExporter) GenerateMOC(title string, nodeIDs []string, g *Graph) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: MOC %s\n", title))
	sb.WriteString("type: moc\n")
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# MOC: %s\n\n", title))
	sb.WriteString("This is a Map of Content for related concepts.\n\n")

	for _, id := range nodeIDs {
		node := g.GetNode(id)
		if node == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- [[%s]]\n", node.Title))
	}

	return sb.String()
}

// escapeYAML 转义 YAML 字符串中的特殊字符
func escapeYAML(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// sanitizeFilename 清理文件名中的非法字符
func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(s)
}

// ParseObsidianMarkdown 解析 Obsidian Markdown 文件为节点
func ParseObsidianMarkdown(content string) (*Node, error) {
	node := &Node{
		Metadata: make(map[string]any),
	}

	// 解析 YAML front matter
	if strings.HasPrefix(content, "---") {
		endIdx := strings.Index(content[3:], "---")
		if endIdx > 0 {
			frontMatter := content[3 : endIdx+3]
			content = content[endIdx+6:]

			for _, line := range strings.Split(frontMatter, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				switch key {
				case "id":
					node.ID = value
				case "title":
					node.Title = value
				case "type":
					node.Type = NodeType(value)
				case "aliases":
					node.Aliases = parseList(value)
				default:
					node.Metadata[key] = value
				}
			}
		}
	}

	// 提取内容中的 Wikilink
	links := ExtractWikilinks(content)
	for _, link := range links {
		node.Outlinks = appendUnique(node.Outlinks, link.Concept)
	}

	node.Content = content
	return node, nil
}

// parseList 解析 YAML 列表格式 [a, b, c]
func parseList(s string) []string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
