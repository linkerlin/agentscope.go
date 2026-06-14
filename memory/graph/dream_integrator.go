package graph

import (
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/memory"
)

// DreamGraphIntegrator 将 Dream 记忆演化与知识图谱结合
type DreamGraphIntegrator struct {
	graph *Graph
}

// NewDreamGraphIntegrator 创建 Dream-Graph 集成器
func NewDreamGraphIntegrator(g *Graph) *DreamGraphIntegrator {
	return &DreamGraphIntegrator{graph: g}
}

// IntegrateDreamDecision 将 Dream 演化决策集成到知识图谱
// 支持 derived_from:: 和 relates_to:: 语义链接
func (dgi *DreamGraphIntegrator) IntegrateDreamDecision(decision *memory.DreamDecision, sourceMemoryID string) error {
	if dgi == nil || dgi.graph == nil || decision == nil || decision.Candidate == nil {
		return fmt.Errorf("invalid integrator or decision")
	}

	cand := decision.Candidate

	// 1. 创建演化节点
	evoNode := &Node{
		ID:      fmt.Sprintf("evo-%d", time.Now().UnixNano()),
		Title:   cand.Bucket,
		Content: cand.Content,
		Type:    NodeTypeMemory,
		Metadata: map[string]any{
			"source_memory": sourceMemoryID,
			"action":        string(decision.Action),
			"score":         cand.Score,
			"when_to_use":   cand.WhenToUse,
			"source_file":   cand.SourceFile,
			"created_at":    time.Now().Format(time.RFC3339),
		},
	}
	if err := dgi.graph.AddNode(evoNode); err != nil {
		return err
	}

	// 2. 创建 derived_from:: 溯源链接
	if sourceMemoryID != "" {
		edge := &Edge{
			Source:        evoNode.ID,
			Target:        sourceMemoryID,
			Relation:      RelDerivedFrom,
			Weight:        1.0,
			Bidirectional: false,
		}
		_ = dgi.graph.AddEdge(edge)
	}

	// 3. 解析 Content 中的 relates_to:: 链接
	relatesToIDs := dgi.extractRelatesTo(cand.Content)
	for _, targetID := range relatesToIDs {
		edge := &Edge{
			Source:        evoNode.ID,
			Target:        targetID,
			Relation:      RelRelatedTo,
			Weight:        0.8,
			Bidirectional: true,
		}
		_ = dgi.graph.AddEdge(edge)
	}

	// 4. 解析 Content 中的 Wikilink 并创建边
	links := ExtractWikilinks(cand.Content)
	for _, link := range links {
		targetID := link.Concept
		if target := dgi.graph.GetNodeByAlias(link.Concept); target != nil {
			targetID = target.ID
		} else if target := dgi.graph.GetNode(link.Concept); target != nil {
			targetID = target.ID
		}

		edge := &Edge{
			Source:        evoNode.ID,
			Target:        targetID,
			Relation:      RelMentions,
			Weight:        0.6,
			Bidirectional: false,
		}
		_ = dgi.graph.AddEdge(edge)
	}

	// 5. 如果决策有 Updated 节点，创建版本追踪链接
	if decision.Updated != nil {
		_ = dgi.graph.AddEdge(&Edge{
			Source:        evoNode.ID,
			Target:        decision.Updated.MemoryID,
			Relation:      RelPartOf,
			Weight:        1.0,
			Bidirectional: false,
		})
	}

	return nil
}

// IntegrateDreamResult 将整个 DreamResult 集成到知识图谱
func (dgi *DreamGraphIntegrator) IntegrateDreamResult(result *memory.DreamResult, sourceMemoryID string) error {
	if dgi == nil || result == nil {
		return nil
	}

	// 集成所有创建的节点
	for _, node := range result.Created {
		decision := &memory.DreamDecision{
			Candidate: &memory.DreamCandidate{
				Bucket:    string(node.MemoryType),
				Content:   node.Content,
				WhenToUse: node.WhenToUse,
				Score:     node.Score,
			},
			Action: memory.DreamCreate,
		}
		_ = dgi.IntegrateDreamDecision(decision, sourceMemoryID)
	}

	// 集成所有精炼的节点
	for _, node := range result.Refined {
		decision := &memory.DreamDecision{
			Candidate: &memory.DreamCandidate{
				Bucket:    string(node.MemoryType),
				Content:   node.Content,
				WhenToUse: node.WhenToUse,
				Score:     node.Score,
			},
			Action:  memory.DreamRefine,
			Updated: node,
		}
		_ = dgi.IntegrateDreamDecision(decision, sourceMemoryID)
	}

	// 集成所有修正的节点
	for _, node := range result.Corrected {
		decision := &memory.DreamDecision{
			Candidate: &memory.DreamCandidate{
				Bucket:    string(node.MemoryType),
				Content:   node.Content,
				WhenToUse: node.WhenToUse,
				Score:     node.Score,
			},
			Action:  memory.DreamCorrect,
			Updated: node,
		}
		_ = dgi.IntegrateDreamDecision(decision, sourceMemoryID)
	}

	return nil
}

// extractRelatesTo 从文本中提取 relates_to:: 链接
// 格式: relates_to:: [[概念]] 或 relates_to:: 概念ID
func (dgi *DreamGraphIntegrator) extractRelatesTo(content string) []string {
	var ids []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "relates_to::") {
			value := strings.TrimSpace(line[len("relates_to::"):])
			// 解析 Wikilink
			links := ExtractWikilinks(value)
			for _, link := range links {
				ids = append(ids, link.Concept)
			}
			// 也支持纯文本 ID
			if len(links) == 0 && value != "" {
				ids = append(ids, value)
			}
		}
	}
	return ids
}

// AutoLinkConcepts 自动将 MemoryNode 中的概念链接到图谱
// 当发现新记忆时，自动创建概念节点和关联边
func (dgi *DreamGraphIntegrator) AutoLinkConcepts(node *memory.MemoryNode) error {
	if dgi == nil || dgi.graph == nil || node == nil {
		return nil
	}

	// 1. 确保记忆节点在图谱中
	memNode := dgi.graph.GetNode(node.MemoryID)
	if memNode == nil {
		memNode = &Node{
			ID:      node.MemoryID,
			Title:   node.MemoryTarget,
			Content: node.Content,
			Type:    NodeTypeMemory,
			Metadata: map[string]any{
				"memory_type": string(node.MemoryType),
				"author":      node.Author,
			},
		}
		_ = dgi.graph.AddNode(memNode)
	}

	// 2. 提取内容中的概念（Wikilink）
	links := ExtractWikilinks(node.Content)
	for _, link := range links {
		// 检查概念节点是否已存在
		conceptNode := dgi.graph.GetNodeByAlias(link.Concept)
		if conceptNode == nil {
			conceptNode = dgi.graph.GetNode(link.Concept)
		}

		// 如果不存在，创建概念节点
		if conceptNode == nil {
			conceptNode = &Node{
				ID:      fmt.Sprintf("concept-%s", sanitizeFilename(link.Concept)),
				Title:   link.Concept,
				Content: "",
				Type:    NodeTypeConcept,
				Aliases: []string{link.Concept},
				Metadata: map[string]any{
					"auto_created": true,
					"created_at":   time.Now().Format(time.RFC3339),
				},
			}
			_ = dgi.graph.AddNode(conceptNode)
		}

		// 创建 mentions 边
		edge := &Edge{
			Source:        node.MemoryID,
			Target:        conceptNode.ID,
			Relation:      RelMentions,
			Weight:        0.7,
			Bidirectional: false,
		}
		_ = dgi.graph.AddEdge(edge)
	}

	return nil
}

// GenerateConceptMap 为指定概念生成概念地图（MOC）
func (dgi *DreamGraphIntegrator) GenerateConceptMap(conceptID string) (*Node, error) {
	if dgi == nil || dgi.graph == nil {
		return nil, fmt.Errorf("invalid integrator")
	}

	concept := dgi.graph.GetNode(conceptID)
	if concept == nil {
		return nil, fmt.Errorf("concept not found: %s", conceptID)
	}

	// 展开 2 层获取相关节点
	related := dgi.graph.Expand(conceptID, 2)

	// 生成 MOC 内容
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Concept Map: %s\n\n", concept.Title))
	sb.WriteString(fmt.Sprintf("## Definition\n\n%s\n\n", concept.Content))

	// 分类整理相关节点
	var memories, concepts, persons, events []*Node
	for _, n := range related {
		if n.ID == conceptID {
			continue
		}
		switch n.Type {
		case NodeTypeMemory:
			memories = append(memories, n)
		case NodeTypeConcept:
			concepts = append(concepts, n)
		case NodeTypePerson:
			persons = append(persons, n)
		case NodeTypeEvent:
			events = append(events, n)
		}
	}

	if len(concepts) > 0 {
		sb.WriteString("## Related Concepts\n\n")
		for _, n := range concepts {
			sb.WriteString(fmt.Sprintf("- [[%s]]\n", n.Title))
		}
		sb.WriteString("\n")
	}

	if len(memories) > 0 {
		sb.WriteString("## Related Memories\n\n")
		for _, n := range memories {
			sb.WriteString(fmt.Sprintf("- [[%s]]: %s\n", n.Title, truncate(n.Content, 100)))
		}
		sb.WriteString("\n")
	}

	if len(persons) > 0 {
		sb.WriteString("## Related People\n\n")
		for _, n := range persons {
			sb.WriteString(fmt.Sprintf("- [[%s]]\n", n.Title))
		}
		sb.WriteString("\n")
	}

	if len(events) > 0 {
		sb.WriteString("## Related Events\n\n")
		for _, n := range events {
			sb.WriteString(fmt.Sprintf("- [[%s]]\n", n.Title))
		}
		sb.WriteString("\n")
	}

	// 生成 MOC 节点
	mocNode := &Node{
		ID:      fmt.Sprintf("moc-%s", conceptID),
		Title:   fmt.Sprintf("MOC %s", concept.Title),
		Content: sb.String(),
		Type:    NodeTypeConcept,
		Metadata: map[string]any{
			"concept_id":   conceptID,
			"auto_created": true,
			"created_at":   time.Now().Format(time.RFC3339),
		},
	}
	_ = dgi.graph.AddNode(mocNode)

	// 创建 MOC 与概念之间的关联
	_ = dgi.graph.AddEdge(&Edge{
		Source:        mocNode.ID,
		Target:        conceptID,
		Relation:      RelPartOf,
		Weight:        1.0,
		Bidirectional: false,
	})

	return mocNode, nil
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
