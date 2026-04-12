package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// Trajectory 执行轨迹，包含消息和成功评分
type Trajectory struct {
	Messages []*message.Msg
	Score    float64 // 0.0 - 1.0，表示执行成功率
	TaskName string
}

// ProceduralSummarizer 从执行轨迹提取任务经验
type ProceduralSummarizer struct {
	Model                model.ChatModel
	Language             string
	SuccessScoreThreshold float64 // 视为成功的阈值，默认0.9
}

// NewProceduralSummarizer 创建任务经验提取器
func NewProceduralSummarizer(m model.ChatModel, lang string) *ProceduralSummarizer {
	if lang == "" {
		lang = "zh"
	}
	return &ProceduralSummarizer{
		Model:                 m,
		Language:              lang,
		SuccessScoreThreshold: 0.9,
	}
}

// ExtractFromTrajectories 从多条轨迹提取任务记忆
func (s *ProceduralSummarizer) ExtractFromTrajectories(ctx context.Context, trajectories []Trajectory) ([]*MemoryNode, error) {
	if s == nil || s.Model == nil || len(trajectories) == 0 {
		return nil, nil
	}

	var allMemories []*MemoryNode

	for _, traj := range trajectories {
		memories, err := s.ExtractFromSingleTrajectory(ctx, traj)
		if err != nil {
			continue
		}
		allMemories = append(allMemories, memories...)
	}

	// 去重
	return s.DeduplicateMemories(allMemories), nil
}

// ExtractFromSingleTrajectory 从单条轨迹提取任务记忆
func (s *ProceduralSummarizer) ExtractFromSingleTrajectory(ctx context.Context, traj Trajectory) ([]*MemoryNode, error) {
	if len(traj.Messages) == 0 {
		return nil, nil
	}

	// 根据评分决定提取策略
	var prompt string
	if traj.Score >= s.SuccessScoreThreshold {
		prompt = s.buildSuccessExtractionPrompt(traj)
	} else if traj.Score < 0.5 {
		prompt = s.buildFailureExtractionPrompt(traj)
	} else {
		// 中等评分，使用对比提取
		prompt = s.buildComparativePrompt(traj)
	}

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	return s.parseTaskMemories(resp.GetTextContent(), traj), nil
}

// ExtractSuccessPattern 从成功轨迹提取有效策略
func (s *ProceduralSummarizer) ExtractSuccessPattern(ctx context.Context, successfulTrajectories []Trajectory) ([]*MemoryNode, error) {
	if len(successfulTrajectories) == 0 {
		return nil, nil
	}

	// 合并多条成功轨迹进行对比学习
	prompt := s.buildMultiSuccessComparisonPrompt(successfulTrajectories)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	return s.parseTaskMemories(resp.GetTextContent(), Trajectory{TaskName: successfulTrajectories[0].TaskName}), nil
}

// ExtractFailureLesson 从失败轨迹提取教训
func (s *ProceduralSummarizer) ExtractFailureLesson(ctx context.Context, failedTrajectories []Trajectory) ([]*MemoryNode, error) {
	if len(failedTrajectories) == 0 {
		return nil, nil
	}

	prompt := s.buildFailureAnalysisPrompt(failedTrajectories)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	return s.parseTaskMemories(resp.GetTextContent(), Trajectory{TaskName: failedTrajectories[0].TaskName, Score: 0}), nil
}

// ValidateMemories 使用LLM验证提取的记忆质量
func (s *ProceduralSummarizer) ValidateMemories(ctx context.Context, memories []*MemoryNode, sampleTrajectory Trajectory) ([]*MemoryNode, []*MemoryNode) {
	if s == nil || s.Model == nil || len(memories) == 0 {
		return memories, nil
	}

	prompt := s.buildValidationPrompt(memories, sampleTrajectory)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return memories, nil
	}

	return s.parseValidationResponse(resp.GetTextContent(), memories)
}

// DeduplicateMemories 基于内容相似度去重
func (s *ProceduralSummarizer) DeduplicateMemories(memories []*MemoryNode) []*MemoryNode {
	if len(memories) <= 1 {
		return memories
	}

	var unique []*MemoryNode
	threshold := 0.85 // 相似度阈值

	for _, m := range memories {
		isDuplicate := false
		for _, u := range unique {
			if s.calculateContentSimilarity(m.Content, u.Content) > threshold {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			unique = append(unique, m)
		}
	}

	return unique
}

// 内部方法

func (s *ProceduralSummarizer) buildSuccessExtractionPrompt(traj Trajectory) string {
	executionLog := s.formatMessages(traj.Messages)

	var sb strings.Builder
	if s.Language == "zh" {
		sb.WriteString("从以下成功的任务执行中提取可复用的经验和最佳实践。\n\n")
		sb.WriteString("执行过程:\n")
		sb.WriteString(executionLog)
		sb.WriteString("\n\n请提取关键经验，格式为JSON数组:\n")
		sb.WriteString(`[{"when_to_use": "何时使用此经验", "memory": "经验内容"}]`)
	} else {
		sb.WriteString("Extract reusable experiences and best practices from the following successful task execution.\n\n")
		sb.WriteString("Execution:\n")
		sb.WriteString(executionLog)
		sb.WriteString("\n\nPlease extract key experiences as JSON array:\n")
		sb.WriteString(`[{"when_to_use": "when to use this experience", "memory": "experience content"}]`)
	}

	return sb.String()
}

func (s *ProceduralSummarizer) buildFailureExtractionPrompt(traj Trajectory) string {
	executionLog := s.formatMessages(traj.Messages)

	var sb strings.Builder
	if s.Language == "zh" {
		sb.WriteString("从以下失败的任务执行中提取教训和需要避免的错误。\n\n")
		sb.WriteString("执行过程:\n")
		sb.WriteString(executionLog)
		sb.WriteString("\n\n请提取关键教训，格式为JSON数组:\n")
		sb.WriteString(`[{"when_to_use": "何时需要注意", "memory": "教训内容"}]`)
	} else {
		sb.WriteString("Extract lessons and mistakes to avoid from the following failed task execution.\n\n")
		sb.WriteString("Execution:\n")
		sb.WriteString(executionLog)
		sb.WriteString("\n\nPlease extract key lessons as JSON array:\n")
		sb.WriteString(`[{"when_to_use": "when to be careful", "memory": "lesson content"}]`)
	}

	return sb.String()
}

func (s *ProceduralSummarizer) buildComparativePrompt(traj Trajectory) string {
	executionLog := s.formatMessages(traj.Messages)

	var sb strings.Builder
	if s.Language == "zh" {
		sb.WriteString("分析以下任务执行（部分成功），提取可以改进的地方。\n\n")
		sb.WriteString("执行过程:\n")
		sb.WriteString(executionLog)
		sb.WriteString("\n\n请提取改进建议，格式为JSON数组:\n")
		sb.WriteString(`[{"when_to_use": "适用场景", "memory": "改进建议"}]`)
	} else {
		sb.WriteString("Analyze the following task execution (partial success) and extract areas for improvement.\n\n")
		sb.WriteString("Execution:\n")
		sb.WriteString(executionLog)
		sb.WriteString("\n\nPlease extract improvement suggestions as JSON array:\n")
		sb.WriteString(`[{"when_to_use": "applicable scenario", "memory": "improvement suggestion"}]`)
	}

	return sb.String()
}

func (s *ProceduralSummarizer) buildMultiSuccessComparisonPrompt(trajectories []Trajectory) string {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("对比以下多条成功的任务执行，提取共同的成功模式和最佳实践。\n\n")
	} else {
		sb.WriteString("Compare the following successful task executions to extract common success patterns.\n\n")
	}

	for i, traj := range trajectories {
		fmt.Fprintf(&sb, "\n=== Execution %d ===\n", i+1)
		sb.WriteString(s.formatMessages(traj.Messages))
	}

	if s.Language == "zh" {
		sb.WriteString("\n\n请提取共同的成功模式，格式为JSON数组:\n")
		sb.WriteString(`[{"when_to_use": "模式适用场景", "memory": "成功模式描述"}]`)
	} else {
		sb.WriteString("\n\nPlease extract common success patterns as JSON array:\n")
		sb.WriteString(`[{"when_to_use": "pattern applicable scenario", "memory": "success pattern description"}]`)
	}

	return sb.String()
}

func (s *ProceduralSummarizer) buildFailureAnalysisPrompt(trajectories []Trajectory) string {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("分析以下失败的执行，提取常见的失败原因和应该避免的错误。\n\n")
	} else {
		sb.WriteString("Analyze the following failed executions to extract common failure causes.\n\n")
	}

	for i, traj := range trajectories {
		fmt.Fprintf(&sb, "\n=== Failed Execution %d ===\n", i+1)
		sb.WriteString(s.formatMessages(traj.Messages))
	}

	if s.Language == "zh" {
		sb.WriteString("\n\n请提取失败教训，格式为JSON数组:\n")
	} else {
		sb.WriteString("\n\nPlease extract failure lessons as JSON array:\n")
	}

	return sb.String()
}

func (s *ProceduralSummarizer) buildValidationPrompt(memories []*MemoryNode, traj Trajectory) string {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("验证以下提取的记忆是否对完成任务真正有帮助。\n\n")
		sb.WriteString("原始执行:\n")
		sb.WriteString(s.formatMessages(traj.Messages))
		sb.WriteString("\n\n待验证的记忆:\n")
	} else {
		sb.WriteString("Validate whether the following extracted memories are truly helpful for task completion.\n\n")
		sb.WriteString("Original execution:\n")
		sb.WriteString(s.formatMessages(traj.Messages))
		sb.WriteString("\n\nMemories to validate:\n")
	}

	for i, m := range memories {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, m.WhenToUse, m.Content)
	}

	if s.Language == "zh" {
		sb.WriteString("\n\n对每个记忆，输出: <序号> <有效|无效>\n")
	} else {
		sb.WriteString("\n\nFor each memory, output: <number> <valid|invalid>\n")
	}

	return sb.String()
}

func (s *ProceduralSummarizer) formatMessages(msgs []*message.Msg) string {
	var lines []string
	for _, m := range msgs {
		role := string(m.Role)
		content := m.GetTextContent()
		lines = append(lines, role+": "+content)
	}
	return strings.Join(lines, "\n")
}

func (s *ProceduralSummarizer) parseTaskMemories(text string, traj Trajectory) []*MemoryNode {
	// 尝试解析JSON格式
	text = s.extractJSONFromMarkdown(text)

	var rawMemories []struct {
		WhenToUse string `json:"when_to_use"`
		Memory    string `json:"memory"`
	}

	if err := json.Unmarshal([]byte(text), &rawMemories); err != nil {
		// 回退到文本解析
		return s.parseTaskMemoriesFromText(text, traj)
	}

	var nodes []*MemoryNode
	for _, raw := range rawMemories {
		if raw.Memory == "" {
			continue
		}

		node := NewMemoryNode(MemoryTypeProcedural, traj.TaskName, raw.Memory)
		node.WhenToUse = raw.WhenToUse
		node.Metadata["source_score"] = traj.Score
		node.Metadata["extraction_time"] = time.Now().Format(time.RFC3339)

		if traj.Score >= s.SuccessScoreThreshold {
			node.Metadata["pattern_type"] = "success"
		} else if traj.Score < 0.5 {
			node.Metadata["pattern_type"] = "failure"
		} else {
			node.Metadata["pattern_type"] = "improvement"
		}

		nodes = append(nodes, node)
	}

	return nodes
}

func (s *ProceduralSummarizer) parseTaskMemoriesFromText(text string, traj Trajectory) []*MemoryNode {
	// 文本格式解析: 经验：<场景> <> <内容>
	pattern := regexp.MustCompile(`(?:经验|Experience|Pattern)：?<([^>]+)>\s*<>\s*<([^>]+)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var nodes []*MemoryNode
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		whenToUse := strings.TrimSpace(match[1])
		content := strings.TrimSpace(match[2])

		if content == "" {
			continue
		}

		node := NewMemoryNode(MemoryTypeProcedural, traj.TaskName, content)
		node.WhenToUse = whenToUse
		node.Metadata["source_score"] = traj.Score
		nodes = append(nodes, node)
	}

	return nodes
}

func (s *ProceduralSummarizer) parseValidationResponse(text string, memories []*MemoryNode) ([]*MemoryNode, []*MemoryNode) {
	// 解析格式: <序号> <有效|无效> 或 <number> <valid|invalid>
	pattern := regexp.MustCompile(`<(\d+)>\s*<(有效|无效|valid|invalid)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	validMap := make(map[int]bool)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		idx := 0
		fmt.Sscanf(match[1], "%d", &idx)
		status := strings.ToLower(match[2])

		if idx > 0 && idx <= len(memories) {
			validMap[idx-1] = status == "有效" || status == "valid"
		}
	}

	var valid, invalid []*MemoryNode
	for i, m := range memories {
		if validMap[i] {
			valid = append(valid, m)
		} else {
			invalid = append(invalid, m)
		}
	}

	return valid, invalid
}

func (s *ProceduralSummarizer) extractJSONFromMarkdown(text string) string {
	// 提取 ```json ... ``` 或 ``` ... ``` 中的内容
	pattern := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	matches := pattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return text
}

func (s *ProceduralSummarizer) calculateContentSimilarity(content1, content2 string) float64 {
	// 简单的Jaccard相似度
	words1 := s.tokenize(content1)
	words2 := s.tokenize(content2)

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

func (s *ProceduralSummarizer) tokenize(text string) []string {
	// 简单的分词和清洗
	text = strings.ToLower(text)
	for _, sep := range []string{"。", "，", ",", ".", "!", "?", "；", "、", "\n", "\t"} {
		text = strings.ReplaceAll(text, sep, " ")
	}

	words := strings.Fields(text)
	var filtered []string
	for _, w := range words {
		if len(w) > 2 { // 过滤短词
			filtered = append(filtered, w)
		}
	}
	return filtered
}
