package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// PersonalSummarizer 从对话提取个人记忆（观察、洞察）
type PersonalSummarizer struct {
	Model    model.ChatModel
	Language string // "zh" or "en"
}

// NewPersonalSummarizer 创建个人记忆提取器
func NewPersonalSummarizer(m model.ChatModel, lang string) *PersonalSummarizer {
	if lang == "" {
		lang = "zh"
	}
	return &PersonalSummarizer{Model: m, Language: lang}
}

// ExtractObservations 从对话提取观察记忆
func (s *PersonalSummarizer) ExtractObservations(ctx context.Context, msgs []*message.Msg, userName string) ([]*MemoryNode, error) {
	if s == nil || s.Model == nil || len(msgs) == 0 {
		return nil, nil
	}

	// 过滤时间相关消息
	filtered := s.filterTimeRelatedMessages(msgs)
	if len(filtered) == 0 {
		return nil, nil
	}

	// 构建提取提示
	prompt := s.buildObservationPrompt(filtered, userName)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	return s.parseObservations(resp.GetTextContent(), filtered, userName), nil
}

// ExtractInsights 从观察提取洞察（更高层次的总结）
func (s *PersonalSummarizer) ExtractInsights(ctx context.Context, observations []*MemoryNode, userName string) ([]*MemoryNode, error) {
	if s == nil || s.Model == nil || len(observations) == 0 {
		return nil, nil
	}

	prompt := s.buildInsightPrompt(observations, userName)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	return s.parseInsights(resp.GetTextContent(), userName), nil
}

// UpdateInsights 基于新观察更新已有洞察
func (s *PersonalSummarizer) UpdateInsights(ctx context.Context, insights, observations []*MemoryNode, userName string) ([]*MemoryNode, error) {
	if s == nil || s.Model == nil || len(insights) == 0 || len(observations) == 0 {
		return insights, nil
	}

	var updated []*MemoryNode

	for _, insight := range insights {
		// 找出与此洞察相关的观察
		relevant := s.findRelevantObservations(insight, observations)
		if len(relevant) == 0 {
			updated = append(updated, insight)
			continue
		}

		// 使用LLM更新洞察
		newContent, err := s.updateInsightWithObservations(ctx, insight, relevant, userName)
		if err != nil || newContent == "" || newContent == insight.Content {
			updated = append(updated, insight)
			continue
		}

		// 创建更新的洞察节点
		updatedInsight := &MemoryNode{
			MemoryID:      insight.MemoryID,
			MemoryType:    MemoryTypePersonal,
			MemoryTarget:  insight.MemoryTarget,
			Content:       newContent,
			WhenToUse:     insight.WhenToUse,
			RefMemoryID:   insight.RefMemoryID,
			TimeCreated:   insight.TimeCreated,
			TimeModified:  time.Now(),
			Author:        insight.Author,
			Metadata:      insight.Metadata,
		}
		updatedInsight.Metadata["updated_by"] = "personal_summarizer"
		updatedInsight.Metadata["original_content"] = insight.Content

		updated = append(updated, updatedInsight)
	}

	return updated, nil
}

// HandleContraRepeat 处理矛盾/重复记忆
// 返回: (保留的记忆列表, 被删除的记忆ID列表)
func (s *PersonalSummarizer) HandleContraRepeat(ctx context.Context, memories []*MemoryNode) ([]*MemoryNode, []string, error) {
	if s == nil || s.Model == nil || len(memories) <= 1 {
		return memories, nil, nil
	}

	// 限制处理数量
	const maxCount = 50
	if len(memories) > maxCount {
		// 按时间倒序，取最近的
		memories = memories[:maxCount]
	}

	prompt := s.buildContraRepeatPrompt(memories)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return memories, nil, err
	}

	return s.parseContraRepeatResponse(resp.GetTextContent(), memories)
}

// 内部方法

func (s *PersonalSummarizer) filterTimeRelatedMessages(msgs []*message.Msg) []*message.Msg {
	timeKeywords := []string{
		"今天", "明天", "昨天", "现在", "刚才", "最近", "上次", "下次",
		"today", "tomorrow", "yesterday", "now", "just now", "recently", "last time", "next time",
	}

	var filtered []*message.Msg
	for _, m := range msgs {
		content := strings.ToLower(m.GetTextContent())
		hasTime := false
		for _, kw := range timeKeywords {
			if strings.Contains(content, strings.ToLower(kw)) {
				hasTime = true
				break
			}
		}
		if !hasTime {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func (s *PersonalSummarizer) buildObservationPrompt(msgs []*message.Msg, userName string) string {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("从以下对话中提取关于用户的持久性信息（偏好、习惯、背景等）。\n\n")
		sb.WriteString("规则:\n")
		sb.WriteString("1. 只提取事实性信息，不猜测\n")
		sb.WriteString("2. 提取的信息应该是长期有效的（非时间相关的）\n")
		sb.WriteString("3. 格式: 信息：<序号> <> <内容> <关键词>\n")
		sb.WriteString("4. 如果没有有效信息，输出: 无\n\n")
		sb.WriteString("示例:\n")
		sb.WriteString("信息：<1> <> <用户喜欢喝咖啡> <饮食偏好>\n")
		sb.WriteString("信息：<2> <> <用户是软件工程师> <职业>\n\n")
		sb.WriteString("对话:\n")
	} else {
		sb.WriteString("Extract persistent information about the user from the following conversation.\n\n")
		sb.WriteString("Rules:\n")
		sb.WriteString("1. Extract factual information only, no guessing\n")
		sb.WriteString("2. Information should be long-term valid (not time-related)\n")
		sb.WriteString("3. Format: Information: <number> <> <content> <keywords>\n")
		sb.WriteString("4. If no valid information, output: None\n\n")
		sb.WriteString("Examples:\n")
		sb.WriteString("Information: <1> <> <User likes coffee> <dietary preference>\n")
		sb.WriteString("Information: <2> <> <User is a software engineer> <profession>\n\n")
		sb.WriteString("Conversation:\n")
	}

	for i, m := range msgs {
		fmt.Fprintf(&sb, "%d %s: %s\n", i+1, userName, m.GetTextContent())
	}

	return sb.String()
}

func (s *PersonalSummarizer) buildInsightPrompt(observations []*MemoryNode, userName string) string {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("基于以下观察，总结用户的深层洞察（性格、价值观、行为模式等）。\n\n")
		sb.WriteString("观察:\n")
		for _, obs := range observations {
			fmt.Fprintf(&sb, "- %s\n", obs.Content)
		}
		sb.WriteString("\n请总结3-5个关键洞察，格式: 洞察：<主题> <> <描述>\n")
	} else {
		sb.WriteString("Based on the following observations, summarize deep insights about the user (personality, values, behavior patterns).\n\n")
		sb.WriteString("Observations:\n")
		for _, obs := range observations {
			fmt.Fprintf(&sb, "- %s\n", obs.Content)
		}
		sb.WriteString("\nPlease summarize 3-5 key insights, format: Insight: <theme> <> <description>\n")
	}

	return sb.String()
}

func (s *PersonalSummarizer) buildContraRepeatPrompt(memories []*MemoryNode) string {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("分析以下记忆列表，识别矛盾或重复的记忆。\n\n")
		sb.WriteString("规则:\n")
		sb.WriteString("- 矛盾：两个记忆内容相互矛盾\n")
		sb.WriteString("- 被包含：一个记忆的内容被另一个完全包含\n")
		sb.WriteString("- 无：记忆有效且独立\n\n")
		sb.WriteString("格式: <序号> <矛盾|被包含|无>\n\n")
		sb.WriteString("记忆列表:\n")
	} else {
		sb.WriteString("Analyze the following memory list to identify contradictory or repetitive memories.\n\n")
		sb.WriteString("Rules:\n")
		sb.WriteString("- Contradiction: Two memories contradict each other\n")
		sb.WriteString("- Contained: One memory is fully contained within another\n")
		sb.WriteString("- None: Memory is valid and independent\n\n")
		sb.WriteString("Format: <number> <Contradiction|Contained|None>\n\n")
		sb.WriteString("Memory list:\n")
	}

	for i, m := range memories {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, m.Content)
	}

	return sb.String()
}

func (s *PersonalSummarizer) parseObservations(text string, msgs []*message.Msg, userName string) []*MemoryNode {
	// 匹配格式: 信息：<序号> <> <内容> <关键词> 或 Information: <number> <> <content> <keywords>
	pattern := regexp.MustCompile(`(?:信息|Information)：?<(\d+)>\s*<>\s*<([^>]+)>\s*<([^>]*)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var nodes []*MemoryNode
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		idx := 0
		fmt.Sscanf(match[1], "%d", &idx)
		content := strings.TrimSpace(match[2])
		keywords := strings.TrimSpace(match[3])

		if content == "" || strings.EqualFold(content, "无") || strings.EqualFold(content, "none") {
			continue
		}

		// 关联到源消息
		var refID string
		if idx > 0 && idx <= len(msgs) {
			refID = msgs[idx-1].ID
		}

		node := NewMemoryNode(MemoryTypePersonal, userName, content)
		node.WhenToUse = keywords
		node.RefMemoryID = refID
		node.Metadata["observation_type"] = "personal_info"
		node.Metadata["keywords"] = keywords

		nodes = append(nodes, node)
	}

	return nodes
}

func (s *PersonalSummarizer) parseInsights(text string, userName string) []*MemoryNode {
	// 匹配格式: 洞察：<主题> <> <描述> 或 Insight: <theme> <> <description>
	pattern := regexp.MustCompile(`(?:洞察|Insight)：?<([^>]+)>\s*<>\s*<([^>]+)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var nodes []*MemoryNode
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		subject := strings.TrimSpace(match[1])
		content := strings.TrimSpace(match[2])

		if content == "" {
			continue
		}

		node := NewMemoryNode(MemoryTypePersonal, userName, content)
		node.WhenToUse = subject
		node.Metadata["insight_subject"] = subject
		node.Metadata["observation_type"] = "insight"

		nodes = append(nodes, node)
	}

	return nodes
}

func (s *PersonalSummarizer) parseContraRepeatResponse(text string, memories []*MemoryNode) ([]*MemoryNode, []string, error) {
	// 匹配格式: <序号> <矛盾|被包含|无> 或 <number> <Contradiction|Contained|None>
	pattern := regexp.MustCompile(`<(\d+)>\s*<(矛盾|被包含|无|Contradiction|Contained|None)>`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	toRemove := make(map[int]bool)
	var deletedIDs []string

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		idx := 0
		fmt.Sscanf(match[1], "%d", &idx)
		judgment := strings.ToLower(match[2])

		if idx > 0 && idx <= len(memories) {
			if judgment == "矛盾" || judgment == "contradiction" ||
				judgment == "被包含" || judgment == "contained" {
				toRemove[idx-1] = true
				deletedIDs = append(deletedIDs, memories[idx-1].MemoryID)
			}
		}
	}

	var filtered []*MemoryNode
	for i, m := range memories {
		if !toRemove[i] {
			filtered = append(filtered, m)
		}
	}

	return filtered, deletedIDs, nil
}

func (s *PersonalSummarizer) findRelevantObservations(insight *MemoryNode, observations []*MemoryNode) []*MemoryNode {
	// 基于关键词匹配找到相关观察
	insightKeywords := s.extractKeywords(insight.Content)
	var relevant []*MemoryNode

	threshold := 0.3 // 最小相关性阈值

	for _, obs := range observations {
		obsKeywords := s.extractKeywords(obs.Content)
		similarity := s.calculateJaccardSimilarity(insightKeywords, obsKeywords)

		// 检查reflection_subject匹配
		insightSubject := insight.Metadata["insight_subject"]
		obsSubject := obs.Metadata["insight_subject"]
		if insightSubject != "" && obsSubject != "" && insightSubject == obsSubject {
			similarity = 0.9 // 高相关性
		}

		if similarity >= threshold {
			relevant = append(relevant, obs)
		}
	}

	return relevant
}

func (s *PersonalSummarizer) updateInsightWithObservations(ctx context.Context, insight *MemoryNode, observations []*MemoryNode, userName string) (string, error) {
	var sb strings.Builder

	if s.Language == "zh" {
		sb.WriteString("基于以下新观察，更新用户洞察。\n\n")
		sb.WriteString("原洞察:\n")
		fmt.Fprintf(&sb, "%s: %s\n\n", insight.Metadata["insight_subject"], insight.Content)
		sb.WriteString("新观察:\n")
		for _, obs := range observations {
			fmt.Fprintf(&sb, "- %s\n", obs.Content)
		}
		sb.WriteString("\n请输出更新后的洞察内容，格式: 的资料：<内容>\n")
	} else {
		sb.WriteString("Based on the following new observations, update the user insight.\n\n")
		sb.WriteString("Original insight:\n")
		fmt.Fprintf(&sb, "%s: %s\n\n", insight.Metadata["insight_subject"], insight.Content)
		sb.WriteString("New observations:\n")
		for _, obs := range observations {
			fmt.Fprintf(&sb, "- %s\n", obs.Content)
		}
		sb.WriteString("\nPlease output the updated insight content, format: 's profile: <content>\n")
	}

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(sb.String()).Build(),
	})
	if err != nil {
		return "", err
	}

	// 解析响应
	text := resp.GetTextContent()
	pattern := regexp.MustCompile(`(?:的资料|profile)[：:]\s*<([^>]+)>`)
	matches := pattern.FindStringSubmatch(text)

	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	return "", nil
}

func (s *PersonalSummarizer) extractKeywords(text string) map[string]struct{} {
	words := strings.Fields(strings.ToLower(text))
	keywords := make(map[string]struct{})
	for _, w := range words {
		if len(w) > 2 { // 过滤短词
			keywords[w] = struct{}{}
		}
	}
	return keywords
}

func (s *PersonalSummarizer) calculateJaccardSimilarity(set1, set2 map[string]struct{}) float64 {
	if len(set1) == 0 || len(set2) == 0 {
		return 0.0
	}

	intersection := 0
	for w := range set1 {
		if _, ok := set2[w]; ok {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
