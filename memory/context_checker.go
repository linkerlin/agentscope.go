package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/memory/vector"
	"github.com/linkerlin/agentscope.go/message"
)

// CheckContext 按 token 阈值划分待压缩前缀与保留后缀；threshold/reserve 均为 token 估算值
func CheckContext(ctx context.Context, msgs []*message.Msg, threshold, reserve int, counter TokenCounter) (*ContextCheckResult, error) {
	_ = ctx
	if counter == nil {
		counter = NewSimpleTokenCounter()
	}
	total, err := counter.CountMessages(msgs)
	if err != nil {
		return nil, err
	}
	res := &ContextCheckResult{
		TotalTokens: total,
		Threshold:   threshold,
		IsValid:     true,
	}
	if total <= threshold {
		res.MessagesToKeep = cloneMsgSlice(msgs)
		return res, nil
	}

	// 从尾部累加，直到达到 reserve tokens，确定分割点 start
	start := len(msgs)
	var acc int
	for i := len(msgs) - 1; i >= 0; i-- {
		chunk, err := counter.CountMessages([]*message.Msg{msgs[i]})
		if err != nil {
			return nil, err
		}
		acc += chunk
		start = i
		if acc >= reserve {
			break
		}
	}
	for start < len(msgs) && !splitRespectsToolPairs(msgs, start) {
		start++
	}
	res.MessagesToCompact = cloneMsgSlice(msgs[:start])
	res.MessagesToKeep = cloneMsgSlice(msgs[start:])
	res.IsValid = splitRespectsToolPairs(msgs, start)
	return res, nil
}

func cloneMsgSlice(msgs []*message.Msg) []*message.Msg {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*message.Msg, len(msgs))
	copy(out, msgs)
	return out
}

// splitRespectsToolPairs compact 前缀内每个 tool_use 在同前缀内均有对应 tool_result
func splitRespectsToolPairs(msgs []*message.Msg, start int) bool {
	if start == 0 {
		return true
	}
	pending := make(map[string]struct{})
	for _, m := range msgs[:start] {
		if m == nil {
			continue
		}
		for _, tc := range m.GetToolUseCalls() {
			pending[tc.ID] = struct{}{}
		}
		for _, tr := range m.GetToolResults() {
			delete(pending, tr.ToolUseID)
		}
	}
	return len(pending) == 0
}

// ContextCompletenessReport 完整性检查报告
type ContextCompletenessReport struct {
	ToolAlignment     *ToolAlignmentCheck `json:"tool_alignment,omitempty"`
	KnowledgeGaps     []KnowledgeGap      `json:"knowledge_gaps,omitempty"`
	SemanticDrift     float64             `json:"semantic_drift"` // 0-1
	MissingReferences []string            `json:"missing_references,omitempty"`
	Recommendations   []string            `json:"recommendations,omitempty"`
}

// ToolAlignmentCheck 工具对齐检查
type ToolAlignmentCheck struct {
	IsAligned       bool           `json:"is_aligned"`
	MissingTools    []string       `json:"missing_tools,omitempty"`
	UnusedTools     []string       `json:"unused_tools,omitempty"`
	MismatchedCalls []ToolMismatch `json:"mismatched_calls,omitempty"`
	Score           float64        `json:"score"` // 0-1
}

// ToolMismatch 工具调用不匹配
type ToolMismatch struct {
	ToolName    string `json:"tool_name"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Description string `json:"description"`
}

// KnowledgeGap 知识缺口
type KnowledgeGap struct {
	Topic      string  `json:"topic"`
	Severity   string  `json:"severity"` // high/medium/low
	Confidence float64 `json:"confidence"`
	Suggestion string  `json:"suggestion,omitempty"`
}

// CheckContextCompleteness 检查上下文完整性（工具对齐 + 知识缺口 + 语义漂移）
func CheckContextCompleteness(ctx context.Context, msgs []*message.Msg, store VectorStore) (*ContextCompletenessReport, error) {
	report := &ContextCompletenessReport{
		Recommendations: []string{},
	}

	// 1. 工具对齐检查
	toolCheck := checkToolAlignment(msgs)
	report.ToolAlignment = toolCheck

	// 2. 知识缺口检测（基于向量检索）
	if store != nil {
		gaps, err := detectKnowledgeGaps(ctx, msgs, store)
		if err == nil {
			report.KnowledgeGaps = gaps
		}
	}

	// 3. 语义漂移检测
	report.SemanticDrift = detectSemanticDrift(msgs)

	// 4. 缺失引用检测
	report.MissingReferences = detectMissingReferences(msgs)

	// 5. 生成建议
	report.Recommendations = generateRecommendations(report)

	return report, nil
}

// checkToolAlignment 检查工具调用对齐
func checkToolAlignment(msgs []*message.Msg) *ToolAlignmentCheck {
	check := &ToolAlignmentCheck{
		IsAligned:       true,
		MissingTools:    []string{},
		UnusedTools:     []string{},
		MismatchedCalls: []ToolMismatch{},
		Score:           1.0,
	}

	// 收集所有工具声明和调用
	declaredTools := make(map[string]string) // name -> description
	calledTools := make(map[string]int)      // name -> count
	results := make(map[string]bool)         // tool_use_id -> has_result

	for _, m := range msgs {
		if m == nil {
			continue
		}

		// 收集工具声明（通常在 system 消息中）
		if m.Role == message.RoleSystem {
			content := m.GetTextContent()
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Tool: ") || strings.HasPrefix(line, "tool: ") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						declaredTools[strings.TrimSpace(parts[1])] = ""
					}
				}
			}
		}

		// 收集工具调用
		for _, tc := range m.GetToolUseCalls() {
			calledTools[tc.Name]++
			results[tc.ID] = false
		}

		// 收集工具结果
		for _, tr := range m.GetToolResults() {
			results[tr.ToolUseID] = true
		}
	}

	// 检查未调用的工具
	for name := range declaredTools {
		if calledTools[name] == 0 {
			check.UnusedTools = append(check.UnusedTools, name)
		}
	}

	// 检查未声明但被调用的工具
	for name := range calledTools {
		if _, declared := declaredTools[name]; !declared {
			check.MissingTools = append(check.MissingTools, name)
		}
	}

	// 检查未完成的工具调用
	for id, hasResult := range results {
		if !hasResult {
			check.MismatchedCalls = append(check.MismatchedCalls, ToolMismatch{
				ToolName:    id,
				Expected:    "tool_result",
				Actual:      "missing",
				Description: "tool_use without corresponding tool_result",
			})
		}
	}

	// 计算分数
	if len(check.MissingTools) > 0 || len(check.MismatchedCalls) > 0 {
		check.IsAligned = false
		check.Score = 1.0 - float64(len(check.MissingTools)+len(check.MismatchedCalls))*0.2
		if check.Score < 0 {
			check.Score = 0
		}
	}

	return check
}

// detectKnowledgeGaps 检测知识缺口
func detectKnowledgeGaps(ctx context.Context, msgs []*message.Msg, store VectorStore) ([]KnowledgeGap, error) {
	gaps := []KnowledgeGap{}

	// 提取所有用户查询中的关键概念
	concepts := extractConcepts(msgs)

	for _, concept := range concepts {
		// 检索相关知识
		results, err := store.Search(ctx, concept, vector.RetrieveOptions{
			TopK:     3,
			MinScore: 0.0,
		})
		if err != nil {
			continue
		}

		// 如果检索结果为空或相关性低，标记为知识缺口
		if len(results) == 0 {
			gaps = append(gaps, KnowledgeGap{
				Topic:      concept,
				Severity:   "high",
				Confidence: 0.9,
				Suggestion: fmt.Sprintf("Consider adding knowledge about '%s' to memory store", concept),
			})
		} else if results[0].Score < 0.5 {
			gaps = append(gaps, KnowledgeGap{
				Topic:      concept,
				Severity:   "medium",
				Confidence: 0.7,
				Suggestion: fmt.Sprintf("Knowledge about '%s' may be insufficient", concept),
			})
		}
	}

	return gaps, nil
}

// extractConcepts 从消息中提取关键概念
func extractConcepts(msgs []*message.Msg) []string {
	concepts := []string{}
	seen := make(map[string]bool)

	for _, m := range msgs {
		if m == nil || m.Role != message.RoleUser {
			continue
		}

		content := m.GetTextContent()
		// 简单提取：按标点分割，过滤短词
		words := strings.FieldsFunc(content, func(r rune) bool {
			return r == ' ' || r == ',' || r == '.' || r == '?' || r == '!'
		})

		for _, w := range words {
			w = strings.ToLower(strings.TrimSpace(w))
			if len(w) > 3 && !seen[w] {
				concepts = append(concepts, w)
				seen[w] = true
			}
		}
	}

	return concepts
}

// detectSemanticDrift 检测语义漂移
func detectSemanticDrift(msgs []*message.Msg) float64 {
	if len(msgs) < 3 {
		return 0.0
	}

	// 简单实现：检测话题变化频率
	topicChanges := 0
	for i := 1; i < len(msgs); i++ {
		if msgs[i] == nil || msgs[i-1] == nil {
			continue
		}

		// 如果连续消息内容差异大，认为有漂移
		contentA := msgs[i].GetTextContent()
		contentB := msgs[i-1].GetTextContent()
		if len(contentA) > 10 && len(contentB) > 10 {
			// 简单 Jaccard 相似度
			sim := jaccardSimilarity(contentA, contentB)
			if sim < 0.1 {
				topicChanges++
			}
		}
	}

	drift := float64(topicChanges) / float64(len(msgs)-1)
	if drift > 1.0 {
		drift = 1.0
	}
	return drift
}

// jaccardSimilarity 计算 Jaccard 相似度
func jaccardSimilarity(a, b string) float64 {
	setA := make(map[string]bool)
	setB := make(map[string]bool)

	for _, w := range strings.Fields(a) {
		setA[strings.ToLower(w)] = true
	}
	for _, w := range strings.Fields(b) {
		setB[strings.ToLower(w)] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// detectMissingReferences 检测缺失的引用
func detectMissingReferences(msgs []*message.Msg) []string {
	missing := []string{}
	seen := make(map[string]bool)

	for _, m := range msgs {
		if m == nil {
			continue
		}

		content := m.GetTextContent()
		// 检测 "根据..."、"引用..." 等模式
		if strings.Contains(content, "根据") || strings.Contains(content, "引用") {
			// 提取引用目标
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.Contains(line, "[") && strings.Contains(line, "]") {
					ref := extractReference(line)
					if ref != "" && !seen[ref] {
						seen[ref] = true
						// 检查引用是否实际存在
						if !isReferenceValid(ref, msgs) {
							missing = append(missing, ref)
						}
					}
				}
			}
		}
	}

	return missing
}

// extractReference 从文本中提取引用
func extractReference(text string) string {
	start := strings.Index(text, "[")
	end := strings.Index(text, "]")
	if start >= 0 && end > start {
		return strings.TrimSpace(text[start+1 : end])
	}
	return ""
}

// isReferenceValid 检查引用是否有效
func isReferenceValid(ref string, msgs []*message.Msg) bool {
	for _, m := range msgs {
		if m == nil {
			continue
		}
		if strings.Contains(m.GetTextContent(), ref) {
			return true
		}
	}
	return false
}

// generateRecommendations 生成建议
func generateRecommendations(report *ContextCompletenessReport) []string {
	recs := []string{}

	if report.ToolAlignment != nil && !report.ToolAlignment.IsAligned {
		if len(report.ToolAlignment.MissingTools) > 0 {
			recs = append(recs, fmt.Sprintf("补充工具定义: %v", report.ToolAlignment.MissingTools))
		}
		if len(report.ToolAlignment.MismatchedCalls) > 0 {
			recs = append(recs, "检查未完成的工具调用")
		}
	}

	if len(report.KnowledgeGaps) > 0 {
		recs = append(recs, fmt.Sprintf("补充 %d 个知识缺口", len(report.KnowledgeGaps)))
	}

	if report.SemanticDrift > 0.5 {
		recs = append(recs, "检测到显著语义漂移，建议进行对话总结")
	}

	if len(report.MissingReferences) > 0 {
		recs = append(recs, fmt.Sprintf("补充 %d 个缺失引用", len(report.MissingReferences)))
	}

	if len(recs) == 0 {
		recs = append(recs, "上下文完整性良好")
	}

	return recs
}
