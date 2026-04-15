package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// MemoryDeduplicator 记忆去重器
type MemoryDeduplicator struct {
	EmbedModel      EmbeddingModel
	SimilarityThreshold float64 // 相似度阈值，默认0.85
	LLM             model.ChatModel // 可选，用于语义去重判断
	Language        string
}

// NewMemoryDeduplicator 创建记忆去重器
func NewMemoryDeduplicator(embed EmbeddingModel) *MemoryDeduplicator {
	return &MemoryDeduplicator{
		EmbedModel:          embed,
		SimilarityThreshold: 0.85,
		Language:            "zh",
	}
}

// WithLLM 设置用于高级去重的LLM
func (d *MemoryDeduplicator) WithLLM(m model.ChatModel) *MemoryDeduplicator {
	d.LLM = m
	return d
}

// Deduplicate 对记忆列表进行去重
// 返回: (保留的记忆列表, 被删除的记忆ID列表)
func (d *MemoryDeduplicator) Deduplicate(ctx context.Context, memories []*MemoryNode) ([]*MemoryNode, []string, error) {
	if len(memories) <= 1 {
		return memories, nil, nil
	}

	// 第一步：基于嵌入向量的相似度去重
	vectorUnique, vectorRemoved := d.deduplicateByVector(ctx, memories)

	// 第二步：如果有LLM，进行语义级别的去重
	if d.LLM != nil && len(vectorUnique) > 1 {
		return d.deduplicateByLLM(ctx, vectorUnique)
	}

	return vectorUnique, vectorRemoved, nil
}

// DeduplicateAgainstStore 将新记忆与已有存储去重
func (d *MemoryDeduplicator) DeduplicateAgainstStore(ctx context.Context, newMemories []*MemoryNode, store VectorStore) ([]*MemoryNode, error) {
	if len(newMemories) == 0 {
		return nil, nil
	}

	var unique []*MemoryNode

	for _, newMem := range newMemories {
		// 生成嵌入
		if len(newMem.Vector) == 0 && d.EmbedModel != nil {
			vec, err := d.EmbedModel.Embed(ctx, newMem.Content)
			if err != nil {
				continue
			}
			newMem.Vector = vec
		}

		// 在存储中搜索相似记忆
		if store != nil && len(newMem.Vector) > 0 {
			similar, err := store.Search(ctx, newMem.Content, RetrieveOptions{
				TopK:     5,
				MinScore: d.SimilarityThreshold,
			})
			if err == nil && len(similar) > 0 {
				// 找到相似记忆，跳过
				continue
			}
		}

		unique = append(unique, newMem)
	}

	return unique, nil
}

// FindContradictions 找出记忆中的矛盾项
func (d *MemoryDeduplicator) FindContradictions(ctx context.Context, memories []*MemoryNode) ([][]*MemoryNode, error) {
	if d.LLM == nil || len(memories) < 2 {
		return nil, nil
	}

	prompt := d.buildContradictionPrompt(memories)

	resp, err := d.LLM.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	return d.parseContradictionResponse(resp.GetTextContent(), memories)
}

// MergeSimilarMemories 合并相似记忆
func (d *MemoryDeduplicator) MergeSimilarMemories(ctx context.Context, memories []*MemoryNode) ([]*MemoryNode, error) {
	if d.LLM == nil || len(memories) <= 1 {
		return memories, nil
	}

	// 按内容相似度分组
	groups := d.groupBySimilarity(memories)

	var merged []*MemoryNode
	for _, group := range groups {
		if len(group) == 1 {
			merged = append(merged, group[0])
			continue
		}

		// 合并组内记忆
		mergedMem, err := d.mergeMemoryGroup(ctx, group)
		if err != nil {
			merged = append(merged, group...) // 合并失败，保留原样
			continue
		}
		merged = append(merged, mergedMem)
	}

	return merged, nil
}

// 内部方法

func (d *MemoryDeduplicator) deduplicateByVector(ctx context.Context, memories []*MemoryNode) ([]*MemoryNode, []string) {
	// 确保所有记忆都有嵌入向量
	for _, m := range memories {
		if len(m.Vector) == 0 && d.EmbedModel != nil {
			vec, err := d.EmbedModel.Embed(ctx, m.Content)
			if err == nil {
				m.Vector = vec
			}
		}
	}

	var unique []*MemoryNode
	var removed []string

	for _, m := range memories {
		if len(m.Vector) == 0 {
			// 无法计算相似度，保留
			unique = append(unique, m)
			continue
		}

		isDuplicate := false
		for _, u := range unique {
			if len(u.Vector) == 0 {
				continue
			}
			sim := CosineSimilarity(m.Vector, u.Vector)
			if sim >= d.SimilarityThreshold {
				isDuplicate = true
				removed = append(removed, m.MemoryID)
				break
			}
		}

		if !isDuplicate {
			unique = append(unique, m)
		}
	}

	return unique, removed
}

func (d *MemoryDeduplicator) deduplicateByLLM(ctx context.Context, memories []*MemoryNode) ([]*MemoryNode, []string, error) {
	prompt := d.buildDeduplicationPrompt(memories)

	resp, err := d.LLM.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return memories, nil, err
	}

	return d.parseDeduplicationResponse(resp.GetTextContent(), memories)
}

func (d *MemoryDeduplicator) groupBySimilarity(memories []*MemoryNode) [][]*MemoryNode {
	visited := make(map[int]bool)
	var groups [][]*MemoryNode

	for i, m1 := range memories {
		if visited[i] {
			continue
		}

		group := []*MemoryNode{m1}
		visited[i] = true

		for j, m2 := range memories {
			if i == j || visited[j] {
				continue
			}

			// 计算相似度
			var sim float64
			if len(m1.Vector) > 0 && len(m2.Vector) > 0 {
				sim = CosineSimilarity(m1.Vector, m2.Vector)
			} else {
				sim = d.calculateTextSimilarity(m1.Content, m2.Content)
			}

			if sim >= d.SimilarityThreshold {
				group = append(group, m2)
				visited[j] = true
			}
		}

		groups = append(groups, group)
	}

	return groups
}

func (d *MemoryDeduplicator) mergeMemoryGroup(ctx context.Context, group []*MemoryNode) (*MemoryNode, error) {
	if len(group) == 0 {
		return nil, fmt.Errorf("empty group")
	}
	if len(group) == 1 {
		return group[0], nil
	}

	var sb strings.Builder
	if d.Language == "zh" {
		sb.WriteString("合并以下相似的记忆，生成一个综合的记忆：\n\n")
	} else {
		sb.WriteString("Merge the following similar memories into one comprehensive memory:\n\n")
	}

	for i, m := range group {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, m.Content)
	}

	resp, err := d.LLM.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(sb.String()).Build(),
	})
	if err != nil {
		return nil, err
	}

	merged := NewMemoryNode(group[0].MemoryType, group[0].MemoryTarget, resp.GetTextContent())
	merged.WhenToUse = group[0].WhenToUse

	// 合并元数据
	merged.Metadata["merged_from"] = len(group)
	for _, m := range group {
		merged.Metadata["source_ids"] = append(merged.Metadata["source_ids"].([]string), m.MemoryID)
	}

	return merged, nil
}

func (d *MemoryDeduplicator) buildDeduplicationPrompt(memories []*MemoryNode) string {
	var sb strings.Builder

	if d.Language == "zh" {
		sb.WriteString("分析以下记忆列表，识别重复或高度相似的记忆。\n\n")
		sb.WriteString("规则:\n")
		sb.WriteString("- 完全包含：一个记忆的内容被另一个完全包含\n")
		sb.WriteString("- 重复：两个记忆表达相同信息\n")
		sb.WriteString("- 相似但不重复：保留两者\n\n")
		sb.WriteString("输出格式: <序号> <重复|包含|保留>\n\n")
	} else {
		sb.WriteString("Analyze the following memory list to identify duplicates or highly similar memories.\n\n")
		sb.WriteString("Rules:\n")
		sb.WriteString("- Contained: One memory is fully contained within another\n")
		sb.WriteString("- Duplicate: Two memories express the same information\n")
		sb.WriteString("- Similar but not duplicate: Keep both\n\n")
		sb.WriteString("Output format: <number> <duplicate|contained|keep>\n\n")
	}

	for i, m := range memories {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, m.WhenToUse, m.Content)
	}

	return sb.String()
}

func (d *MemoryDeduplicator) buildContradictionPrompt(memories []*MemoryNode) string {
	var sb strings.Builder

	if d.Language == "zh" {
		sb.WriteString("分析以下记忆，找出相互矛盾的项。\n\n")
		sb.WriteString("矛盾定义：两个记忆不能同时为真。\n\n")
		sb.WriteString("输出格式（每组矛盾）:\n")
		sb.WriteString("矛盾: <序号1>, <序号2>\n")
		sb.WriteString("原因: <矛盾原因>\n\n")
	} else {
		sb.WriteString("Analyze the following memories to find contradictory pairs.\n\n")
		sb.WriteString("Contradiction definition: Two memories cannot both be true.\n\n")
		sb.WriteString("Output format (for each contradiction):\n")
		sb.WriteString("Contradiction: <number1>, <number2>\n")
		sb.WriteString("Reason: <contradiction reason>\n\n")
	}

	for i, m := range memories {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, m.Content)
	}

	return sb.String()
}

func (d *MemoryDeduplicator) parseDeduplicationResponse(text string, memories []*MemoryNode) ([]*MemoryNode, []string, error) {
	// 解析格式: <序号> <重复|包含|保留> 或 <number> <duplicate|contained|keep>
	pattern := regexp.MustCompile(`<(\d+)>\s*<(重复|包含|保留|duplicate|contained|keep)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	toRemove := make(map[int]bool)
	var removed []string

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		idx := 0
		fmt.Sscanf(match[1], "%d", &idx)
		action := strings.ToLower(match[2])

		if idx > 0 && idx <= len(memories) {
			if action == "重复" || action == "duplicate" ||
				action == "包含" || action == "contained" {
				toRemove[idx-1] = true
				removed = append(removed, memories[idx-1].MemoryID)
			}
		}
	}

	var filtered []*MemoryNode
	for i, m := range memories {
		if !toRemove[i] {
			filtered = append(filtered, m)
		}
	}

	return filtered, removed, nil
}

func (d *MemoryDeduplicator) parseContradictionResponse(text string, memories []*MemoryNode) ([][]*MemoryNode, error) {
	// 解析格式: 矛盾: <序号1>, <序号2>
	pattern := regexp.MustCompile(`(?:矛盾|Contradiction)[：:]\s*<(\d+)>[,，]?\s*<(\d+)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var contradictions [][]*MemoryNode
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		idx1, idx2 := 0, 0
		fmt.Sscanf(match[1], "%d", &idx1)
		fmt.Sscanf(match[2], "%d", &idx2)

		if idx1 > 0 && idx1 <= len(memories) && idx2 > 0 && idx2 <= len(memories) {
			contradictions = append(contradictions, []*MemoryNode{
				memories[idx1-1],
				memories[idx2-1],
			})
		}
	}

	return contradictions, nil
}

func (d *MemoryDeduplicator) calculateTextSimilarity(text1, text2 string) float64 {
	// Jaccard相似度
	words1 := d.tokenize(text1)
	words2 := d.tokenize(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	set1 := make(map[string]struct{})
	for _, w := range words1 {
		set1[w] = struct{}{}
	}

	intersection := 0
	for _, w := range words2 {
		if _, ok := set1[w]; ok {
			intersection++
		}
	}

	union := len(words1) + len(words2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (d *MemoryDeduplicator) tokenize(text string) []string {
	text = strings.ToLower(text)
	for _, sep := range []string{"。", "，", ",", ".", "!", "?", "；", "、", "\n", "\t"} {
		text = strings.ReplaceAll(text, sep, " ")
	}

	words := strings.Fields(text)
	var filtered []string
	for _, w := range words {
		if len(w) > 2 {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// SimpleDeduplicate 简单的基于文本相似度的去重（无需嵌入模型）
func SimpleDeduplicate(memories []*MemoryNode, threshold float64) ([]*MemoryNode, []string) {
	if len(memories) <= 1 {
		return memories, nil
	}

	d := &MemoryDeduplicator{SimilarityThreshold: threshold}

	var unique []*MemoryNode
	var removed []string

	for _, m := range memories {
		isDuplicate := false
		for _, u := range unique {
			sim := d.calculateTextSimilarity(m.Content, u.Content)
			if sim >= threshold {
				isDuplicate = true
				removed = append(removed, m.MemoryID)
				break
			}
		}

		if !isDuplicate {
			unique = append(unique, m)
		}
	}

	return unique, removed
}
