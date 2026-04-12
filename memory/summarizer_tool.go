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

// ToolCallResult 工具调用结果记录
type ToolCallResult struct {
	CreateTime   time.Time
	ToolName     string
	Input        map[string]any
	Output       string
	TokenCost    int
	Success      bool
	TimeCost     float64 // 执行耗时（秒）
	IsSummarized bool    // 是否已被总结
	Summary      string  // LLM生成的调用摘要
	Evaluation   string  // LLM对调用质量的评价
	Score        float64 // 评分 0.0-1.0
}

// ToolSummarizer 生成工具使用指南
type ToolSummarizer struct {
	Model            model.ChatModel
	Language         string
	RecentCallCount  int     // 分析最近N条调用记录，默认30
	SummaryInterval  float64 // 总结间隔（秒），用于并发控制
}

// NewToolSummarizer 创建工具记忆总结器
func NewToolSummarizer(m model.ChatModel, lang string) *ToolSummarizer {
	if lang == "" {
		lang = "zh"
	}
	return &ToolSummarizer{
		Model:           m,
		Language:        lang,
		RecentCallCount: 30,
		SummaryInterval: 1.0,
	}
}

// EvaluateToolCall 使用LLM评估单次工具调用
func (s *ToolSummarizer) EvaluateToolCall(ctx context.Context, result *ToolCallResult) error {
	if s == nil || s.Model == nil {
		return nil
	}

	prompt := s.buildEvaluationPrompt(result)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return err
	}

	summary, evaluation, score := s.parseEvaluationResponse(resp.GetTextContent())
	result.Summary = summary
	result.Evaluation = evaluation
	result.Score = score

	return nil
}

// SummarizeToolUsage 从工具调用历史生成使用指南
func (s *ToolSummarizer) SummarizeToolUsage(ctx context.Context, toolName string, results []ToolCallResult) (*MemoryNode, error) {
	if s == nil || s.Model == nil || len(results) == 0 {
		return nil, nil
	}

	// 过滤未总结的记录
	var unsummarized []ToolCallResult
	for _, r := range results {
		if !r.IsSummarized {
			unsummarized = append(unsummarized, r)
		}
	}

	if len(unsummarized) == 0 {
		return nil, nil // 没有新记录需要总结
	}

	// 限制分析数量
	if len(unsummarized) > s.RecentCallCount {
		unsummarized = unsummarized[len(unsummarized)-s.RecentCallCount:]
	}

	// 计算统计信息
	stats := s.calculateStatistics(unsummarized)

	// 构建总结提示
	prompt := s.buildSummaryPrompt(toolName, unsummarized, stats)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return nil, err
	}

	// 生成使用指南
	guide := resp.GetTextContent()
	content := s.formatGuideWithStats(guide, stats)

	// 创建记忆节点
	node := NewMemoryNode(MemoryTypeTool, toolName, content)
	node.WhenToUse = toolName
	node.Metadata["tool_name"] = toolName
	node.Metadata["total_calls"] = len(results)
	node.Metadata["success_rate"] = stats.SuccessRate
	node.Metadata["avg_score"] = stats.AvgScore
	node.Metadata["avg_time_cost"] = stats.AvgTimeCost
	node.Metadata["avg_token_cost"] = stats.AvgTokenCost

	// 标记已总结
	for i := range results {
		results[i].IsSummarized = true
	}

	return node, nil
}

// BatchEvaluate 批量评估工具调用
func (s *ToolSummarizer) BatchEvaluate(ctx context.Context, results []*ToolCallResult) error {
	for _, r := range results {
		if err := s.EvaluateToolCall(ctx, r); err != nil {
			continue
		}
	}
	return nil
}

// GenerateBestPractices 生成工具最佳实践文档
func (s *ToolSummarizer) GenerateBestPractices(ctx context.Context, toolName string, successfulResults []ToolCallResult) (string, error) {
	if len(successfulResults) == 0 {
		return "", nil
	}

	prompt := s.buildBestPracticesPrompt(toolName, successfulResults)

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return "", err
	}

	return resp.GetTextContent(), nil
}

// ToolStatistics 工具使用统计
type ToolStatistics struct {
	TotalCalls    int
	SuccessCount  int
	SuccessRate   float64
	AvgScore      float64
	AvgTimeCost   float64
	AvgTokenCost  float64
	MinTimeCost   float64
	MaxTimeCost   float64
	MinTokenCost  int
	MaxTokenCost  int
}

// 内部方法

func (s *ToolSummarizer) calculateStatistics(results []ToolCallResult) ToolStatistics {
	stats := ToolStatistics{
		TotalCalls:   len(results),
		MinTimeCost:  999999,
		MinTokenCost: 999999,
	}

	var totalScore, totalTime, totalTokens float64

	for _, r := range results {
		if r.Success {
			stats.SuccessCount++
		}
		totalScore += r.Score
		totalTime += r.TimeCost
		totalTokens += float64(r.TokenCost)

		if r.TimeCost < stats.MinTimeCost {
			stats.MinTimeCost = r.TimeCost
		}
		if r.TimeCost > stats.MaxTimeCost {
			stats.MaxTimeCost = r.TimeCost
		}
		if r.TokenCost < stats.MinTokenCost {
			stats.MinTokenCost = r.TokenCost
		}
		if r.TokenCost > stats.MaxTokenCost {
			stats.MaxTokenCost = r.TokenCost
		}
	}

	if stats.TotalCalls > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCalls)
		stats.AvgScore = totalScore / float64(stats.TotalCalls)
		stats.AvgTimeCost = totalTime / float64(stats.TotalCalls)
		stats.AvgTokenCost = totalTokens / float64(stats.TotalCalls)
	}

	return stats
}

func (s *ToolSummarizer) buildEvaluationPrompt(result *ToolCallResult) string {
	var sb strings.Builder

	inputJSON, _ := json.Marshal(result.Input)
	inputStr := string(inputJSON)

	if s.Language == "zh" {
		sb.WriteString("评估以下工具调用，提供摘要、评价和评分。\n\n")
		fmt.Fprintf(&sb, "工具: %s\n", result.ToolName)
		fmt.Fprintf(&sb, "输入: %s\n", inputStr)
		fmt.Fprintf(&sb, "输出: %s\n", truncateString(result.Output, 500))
		fmt.Fprintf(&sb, "成功: %v\n", result.Success)
		fmt.Fprintf(&sb, "耗时: %.2f秒\n", result.TimeCost)
		sb.WriteString("\n输出格式:\n")
		sb.WriteString("摘要：<简要描述调用目的和结果>\n")
		sb.WriteString("评价：<质量评价，包括参数选择和结果质量>\n")
		sb.WriteString("评分：<0.0-1.0之间的数字>\n")
	} else {
		sb.WriteString("Evaluate the following tool call and provide summary, evaluation, and score.\n\n")
		fmt.Fprintf(&sb, "Tool: %s\n", result.ToolName)
		fmt.Fprintf(&sb, "Input: %s\n", inputStr)
		fmt.Fprintf(&sb, "Output: %s\n", truncateString(result.Output, 500))
		fmt.Fprintf(&sb, "Success: %v\n", result.Success)
		fmt.Fprintf(&sb, "Time: %.2fs\n", result.TimeCost)
		sb.WriteString("\nOutput format:\n")
		sb.WriteString("Summary: <brief description of call purpose and result>\n")
		sb.WriteString("Evaluation: <quality evaluation including parameter choice and result quality>\n")
		sb.WriteString("Score: <number between 0.0-1.0>\n")
	}

	return sb.String()
}

func (s *ToolSummarizer) buildSummaryPrompt(toolName string, results []ToolCallResult, stats ToolStatistics) string {
	var sb strings.Builder

	if s.Language == "zh" {
		fmt.Fprintf(&sb, "基于以下%s工具的调用历史（最近%d次），生成使用指南。\n\n", toolName, len(results))
		sb.WriteString("统计信息:\n")
		fmt.Fprintf(&sb, "- 成功率: %.1f%%\n", stats.SuccessRate*100)
		fmt.Fprintf(&sb, "- 平均评分: %.2f\n", stats.AvgScore)
		fmt.Fprintf(&sb, "- 平均耗时: %.2f秒\n", stats.AvgTimeCost)
		fmt.Fprintf(&sb, "- 平均Token消耗: %.0f\n", stats.AvgTokenCost)
		sb.WriteString("\n调用记录:\n")
	} else {
		fmt.Fprintf(&sb, "Based on the following usage history of %s tool (recent %d calls), generate a usage guide.\n\n", toolName, len(results))
		sb.WriteString("Statistics:\n")
		fmt.Fprintf(&sb, "- Success Rate: %.1f%%\n", stats.SuccessRate*100)
		fmt.Fprintf(&sb, "- Average Score: %.2f\n", stats.AvgScore)
		fmt.Fprintf(&sb, "- Average Time: %.2fs\n", stats.AvgTimeCost)
		fmt.Fprintf(&sb, "- Average Token Cost: %.0f\n", stats.AvgTokenCost)
		sb.WriteString("\nCall Records:\n")
	}

	for i, r := range results {
		inputJSON, _ := json.Marshal(r.Input)
		fmt.Fprintf(&sb, "\n### Call %d\n", i+1)
		fmt.Fprintf(&sb, "- Input: %s\n", string(inputJSON))
		fmt.Fprintf(&sb, "- Summary: %s\n", r.Summary)
		fmt.Fprintf(&sb, "- Evaluation: %s\n", r.Evaluation)
		fmt.Fprintf(&sb, "- Score: %.2f\n", r.Score)
		fmt.Fprintf(&sb, "- Success: %v\n", r.Success)
	}

	if s.Language == "zh" {
		sb.WriteString("\n\n请生成使用指南，包括:\n")
		sb.WriteString("1. 工具的最佳使用场景\n")
		sb.WriteString("2. 参数优化建议\n")
		sb.WriteString("3. 常见错误和避免方法\n")
		sb.WriteString("4. 性能优化技巧\n")
	} else {
		sb.WriteString("\n\nPlease generate a usage guide including:\n")
		sb.WriteString("1. Best use cases for the tool\n")
		sb.WriteString("2. Parameter optimization suggestions\n")
		sb.WriteString("3. Common mistakes and how to avoid them\n")
		sb.WriteString("4. Performance optimization tips\n")
	}

	return sb.String()
}

func (s *ToolSummarizer) buildBestPracticesPrompt(toolName string, results []ToolCallResult) string {
	var sb strings.Builder

	if s.Language == "zh" {
		fmt.Fprintf(&sb, "从以下%s工具的成功调用中提取最佳实践。\n\n", toolName)
	} else {
		fmt.Fprintf(&sb, "Extract best practices from the following successful calls of %s tool.\n\n", toolName)
	}

	for i, r := range results {
		inputJSON, _ := json.Marshal(r.Input)
		fmt.Fprintf(&sb, "\n### Success Case %d\n", i+1)
		fmt.Fprintf(&sb, "- Input: %s\n", string(inputJSON))
		fmt.Fprintf(&sb, "- Summary: %s\n", r.Summary)
	}

	if s.Language == "zh" {
		sb.WriteString("\n\n请总结最佳实践和使用模式。")
	} else {
		sb.WriteString("\n\nPlease summarize best practices and usage patterns.")
	}

	return sb.String()
}

func (s *ToolSummarizer) parseEvaluationResponse(text string) (summary, evaluation string, score float64) {
	// 解析格式
	// 摘要：xxx
	// 评价：xxx
	// 评分：0.8

	summaryPattern := regexp.MustCompile(`(?:摘要|Summary)[:：]\s*(.+?)(?:\n|$)`)
	evalPattern := regexp.MustCompile(`(?:评价|Evaluation)[:：]\s*(.+?)(?:\n|$)`)
	scorePattern := regexp.MustCompile(`(?:评分|Score)[:：]\s*(\d+\.?\d*)`)

	summaryMatch := summaryPattern.FindStringSubmatch(text)
	if len(summaryMatch) > 1 {
		summary = strings.TrimSpace(summaryMatch[1])
	}

	evalMatch := evalPattern.FindStringSubmatch(text)
	if len(evalMatch) > 1 {
		evaluation = strings.TrimSpace(evalMatch[1])
	}

	scoreMatch := scorePattern.FindStringSubmatch(text)
	if len(scoreMatch) > 1 {
		fmt.Sscanf(scoreMatch[1], "%f", &score)
	}

	return
}

func (s *ToolSummarizer) formatGuideWithStats(guide string, stats ToolStatistics) string {
	var sb strings.Builder

	sb.WriteString(guide)
	sb.WriteString("\n\n## Statistics\n")

	if s.Language == "zh" {
		fmt.Fprintf(&sb, "- 成功率: %.1f%%\n", stats.SuccessRate*100)
		fmt.Fprintf(&sb, "- 平均评分: %.2f\n", stats.AvgScore)
		fmt.Fprintf(&sb, "- 平均耗时: %.2f秒 (范围: %.2f-%.2f)\n", stats.AvgTimeCost, stats.MinTimeCost, stats.MaxTimeCost)
		fmt.Fprintf(&sb, "- 平均Token消耗: %.0f (范围: %d-%d)\n", stats.AvgTokenCost, stats.MinTokenCost, stats.MaxTokenCost)
	} else {
		fmt.Fprintf(&sb, "- Success Rate: %.1f%%\n", stats.SuccessRate*100)
		fmt.Fprintf(&sb, "- Average Score: %.2f\n", stats.AvgScore)
		fmt.Fprintf(&sb, "- Average Time: %.2fs (range: %.2f-%.2f)\n", stats.AvgTimeCost, stats.MinTimeCost, stats.MaxTimeCost)
		fmt.Fprintf(&sb, "- Average Token Cost: %.0f (range: %d-%d)\n", stats.AvgTokenCost, stats.MinTokenCost, stats.MaxTokenCost)
	}

	return sb.String()
}

// 辅助函数

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
