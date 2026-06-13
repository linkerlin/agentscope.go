package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// ReactReplayExtractor ReAct 复盘提取器
// 分析 ReAct 步历史，提取结构化记忆
type ReactReplayExtractor struct {
	Orchestrator any // 记忆编排器（通过接口解耦）
	Config       ReactReplayConfig
}

// ReactReplayConfig 复盘配置
type ReactReplayConfig struct {
	EnableSuccessPath    bool          // 提取成功路径
	EnableFailureLesson  bool          // 提取失败教训
	EnableNewKnowledge   bool          // 提取新知识
	MinConfidence        float64       // 最低置信度
	MaxExtractedMemories int           // 最大提取记忆数
	AutoSummarize        bool          // 自动调用 Summarize
	AsyncMode            bool          // 异步模式（提交到任务队列）
}

// DefaultReactReplayConfig 返回默认复盘配置
func DefaultReactReplayConfig() ReactReplayConfig {
	return ReactReplayConfig{
		EnableSuccessPath:    true,
		EnableFailureLesson:  true,
		EnableNewKnowledge:   true,
		MinConfidence:        0.7,
		MaxExtractedMemories: 10,
		AutoSummarize:        true,
		AsyncMode:            false,
	}
}

// ReactReplayResult 复盘结果
type ReactReplayResult struct {
	SessionID       string            `json:"session_id"`
	SuccessPath     []*ReplayStep     `json:"success_path,omitempty"`
	FailureLessons  []*ReplayLesson   `json:"failure_lessons,omitempty"`
	NewKnowledge    []*MemoryNode     `json:"new_knowledge,omitempty"`
	ExtractedAt     time.Time         `json:"extracted_at"`
	Confidence      float64           `json:"confidence"`
}

// ReplayStep 成功路径步骤
type ReplayStep struct {
	Iteration   int                    `json:"iteration"`
	Type        ReactStepType          `json:"type"`
	ToolName    string                 `json:"tool_name,omitempty"`
	ToolInput   map[string]any         `json:"tool_input,omitempty"`
	Result      string                 `json:"result,omitempty"`
	MemoryNodes []*MemoryNode          `json:"memory_nodes,omitempty"`
	Confidence  float64                `json:"confidence"`
}

// ReplayLesson 失败教训
type ReplayLesson struct {
	Iteration   int     `json:"iteration"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Suggestion  string  `json:"suggestion"`
	Confidence  float64 `json:"confidence"`
}

// NewReactReplayExtractor 创建复盘提取器
func NewReactReplayExtractor(config ReactReplayConfig) *ReactReplayExtractor {
	return &ReactReplayExtractor{
		Config: config,
	}
}

// Replay 复盘 ReAct 循环，提取结构化记忆
func (r *ReactReplayExtractor) Replay(ctx context.Context, steps []*ReactStep) (*ReactReplayResult, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps to replay")
	}

	result := &ReactReplayResult{
		SessionID:  steps[0].ID[:8], // 取前8位作为 session ID
		ExtractedAt: time.Now(),
	}

	// 1. 提取成功路径
	if r.Config.EnableSuccessPath {
		result.SuccessPath = r.extractSuccessPath(steps)
	}

	// 2. 提取失败教训
	if r.Config.EnableFailureLesson {
		result.FailureLessons = r.extractFailureLessons(steps)
	}

	// 3. 提取新知识
	if r.Config.EnableNewKnowledge {
		result.NewKnowledge = r.extractNewKnowledge(steps)
	}

	// 4. 计算整体置信度
	result.Confidence = r.calculateConfidence(result)

	// 5. 限制提取数量
	if len(result.NewKnowledge) > r.Config.MaxExtractedMemories {
		result.NewKnowledge = result.NewKnowledge[:r.Config.MaxExtractedMemories]
	}

	return result, nil
}

// extractSuccessPath 提取成功路径
func (r *ReactReplayExtractor) extractSuccessPath(steps []*ReactStep) []*ReplayStep {
	var path []*ReplayStep

	for _, step := range steps {
		if step.Type != StepActing {
			continue
		}

		for _, tc := range step.ToolCalls {
			// 检查工具调用是否成功（有对应的 observation 且没有错误）
			if r.isToolCallSuccessful(steps, step.Iteration, tc.ID) {
				path = append(path, &ReplayStep{
					Iteration:  step.Iteration,
					Type:       StepActing,
					ToolName:   tc.Name,
					ToolInput:  tc.Input,
					Confidence: 0.9, // 成功路径高置信度
				})
			}
		}
	}

	return path
}

// isToolCallSuccessful 检查工具调用是否成功
func (r *ReactReplayExtractor) isToolCallSuccessful(steps []*ReactStep, iteration int, toolUseID string) bool {
	for _, step := range steps {
		if step.Iteration != iteration || step.Type != StepObservation {
			continue
		}

		for _, msg := range step.Messages {
			for _, tr := range msg.GetToolResults() {
				if tr.ToolUseID == toolUseID && !tr.IsError {
					return true
				}
			}
		}
	}
	return false
}

// extractFailureLessons 提取失败教训
func (r *ReactReplayExtractor) extractFailureLessons(steps []*ReactStep) []*ReplayLesson {
	var lessons []*ReplayLesson

	for _, step := range steps {
		if step.Type != StepObservation {
			continue
		}

		for _, msg := range step.Messages {
			for _, tr := range msg.GetToolResults() {
				if tr.IsError {
					lesson := r.analyzeError(step.Iteration, tr)
					if lesson != nil {
						lessons = append(lessons, lesson)
					}
				}
			}
		}
	}

	return lessons
}

// analyzeError 分析错误生成教训
func (r *ReactReplayExtractor) analyzeError(iteration int, tr *message.ToolResultBlock) *ReplayLesson {
	content := tr.Content
	var errorText string
	for _, block := range content {
		if tb, ok := block.(*message.TextBlock); ok {
			errorText = tb.Text
			break
		}
	}

	if errorText == "" {
		return nil
	}

	// 简单错误分类
	lesson := &ReplayLesson{
		Iteration:   iteration,
		Type:        "tool_error",
		Description: fmt.Sprintf("Tool %s failed: %s", tr.ToolUseID, errorText),
		Confidence:  0.8,
	}

	// 生成建议
	if strings.Contains(errorText, "not found") || strings.Contains(errorText, "404") {
		lesson.Suggestion = "Verify the resource exists before calling the tool"
		lesson.Type = "resource_not_found"
	} else if strings.Contains(errorText, "timeout") || strings.Contains(errorText, "deadline") {
		lesson.Suggestion = "Consider retrying with longer timeout or checking service health"
		lesson.Type = "timeout"
	} else if strings.Contains(errorText, "permission") || strings.Contains(errorText, "unauthorized") {
		lesson.Suggestion = "Check permissions and authentication before calling"
		lesson.Type = "permission_denied"
	} else {
		lesson.Suggestion = "Review tool input parameters and try again"
	}

	return lesson
}

// extractNewKnowledge 从 observation 中提取新知识
func (r *ReactReplayExtractor) extractNewKnowledge(steps []*ReactStep) []*MemoryNode {
	var knowledge []*MemoryNode
	seen := make(map[string]bool)

	for _, step := range steps {
		if step.Type != StepObservation && step.Type != StepFinal {
			continue
		}

		for _, msg := range step.Messages {
			content := msg.GetTextContent()
			if content == "" {
				continue
			}

			// 简单提取：将 observation 内容作为新知识
			// 实际应使用 NLP 提取关键事实
			keyFacts := r.extractKeyFacts(content)
			for _, fact := range keyFacts {
				if seen[fact] || len(fact) < 10 {
					continue
				}
				seen[fact] = true

				node := NewMemoryNode(MemoryTypeHistory, "react_replay", fact)
				node.Score = 0.8
				node.Metadata["source"] = "react_replay"
				node.Metadata["iteration"] = step.Iteration
				node.Metadata["confidence"] = 0.7
				knowledge = append(knowledge, node)
			}
		}
	}

	return knowledge
}

// extractKeyFacts 从文本中提取关键事实
func (r *ReactReplayExtractor) extractKeyFacts(text string) []string {
	// 简单实现：按句子分割，过滤短句
	var facts []string
	sentences := strings.Split(text, ".")
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) > 20 && len(s) < 500 {
			facts = append(facts, s)
		}
	}
	return facts
}

// calculateConfidence 计算整体置信度
func (r *ReactReplayExtractor) calculateConfidence(result *ReactReplayResult) float64 {
	var totalConfidence float64
	var count int

	for _, step := range result.SuccessPath {
		totalConfidence += step.Confidence
		count++
	}

	for _, lesson := range result.FailureLessons {
		totalConfidence += lesson.Confidence
		count++
	}

	for _, node := range result.NewKnowledge {
		if conf, ok := node.Metadata["confidence"].(float64); ok {
			totalConfidence += conf
			count++
		}
	}

	if count == 0 {
		return 0.5
	}

	return totalConfidence / float64(count)
}

// ToSummarizeResult 将复盘结果转换为 SummarizeResult
func (r *ReactReplayExtractor) ToSummarizeResult(result *ReactReplayResult) *SummarizeResult {
	if result == nil {
		return nil
	}

	sr := &SummarizeResult{
		UpdatedProfiles: make(map[string]map[string]any),
	}

	// 成功路径 → procedural 记忆
	for _, step := range result.SuccessPath {
		if step.ToolName != "" {
			node := NewMemoryNode(MemoryTypeProcedural, step.ToolName, fmt.Sprintf("Successfully used %s with input %v", step.ToolName, step.ToolInput))
			node.Score = step.Confidence
			sr.ProceduralMemories = append(sr.ProceduralMemories, node)
		}
	}

	// 新知识 → history 记忆
	for _, node := range result.NewKnowledge {
		node.MemoryType = MemoryTypeHistory
		sr.AddedHistory = node
		break // 只取第一个作为 AddedHistory
	}

	return sr
}

// ReplayAsync 异步复盘（提交到任务队列）
func (r *ReactReplayExtractor) ReplayAsync(ctx context.Context, queue *AsyncTaskQueue, steps []*ReactStep) string {
	if queue == nil {
		return ""
	}

	// 将复盘任务提交到异步队列
	return queue.Submit(&AsyncTask{
		Type:       TaskTypeIndex,
		Priority:   3,
		Payload:    steps,
		MaxRetries: 2,
	})
}

// ReactReplayStats 复盘统计
type ReactReplayStats struct {
	TotalSessions    int     `json:"total_sessions"`
	SuccessPaths     int     `json:"success_paths"`
	FailureLessons   int     `json:"failure_lessons"`
	NewKnowledge     int     `json:"new_knowledge"`
	AvgConfidence    float64 `json:"avg_confidence"`
	LastReplayTime   time.Time `json:"last_replay_time"`
}

// Stats 返回复盘统计
func (r *ReactReplayExtractor) Stats() *ReactReplayStats {
	return &ReactReplayStats{
		LastReplayTime: time.Now(),
	}
}

// ReactReplayFormatter 复盘结果格式化器
type ReactReplayFormatter struct{}

// FormatMarkdown 格式化为 Markdown 报告
func (f *ReactReplayFormatter) FormatMarkdown(result *ReactReplayResult) string {
	if result == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# ReAct Replay Report\n\n")
	sb.WriteString(fmt.Sprintf("**Session:** %s\n\n", result.SessionID))
	sb.WriteString(fmt.Sprintf("**Confidence:** %.2f\n\n", result.Confidence))
	sb.WriteString(fmt.Sprintf("**Extracted At:** %s\n\n", result.ExtractedAt.Format("2006-01-02 15:04:05")))

	// 成功路径
	if len(result.SuccessPath) > 0 {
		sb.WriteString("## Success Path\n\n")
		for i, step := range result.SuccessPath {
			sb.WriteString(fmt.Sprintf("%d. **Iteration %d**: Tool `%s` (confidence: %.2f)\n", i+1, step.Iteration, step.ToolName, step.Confidence))
			if len(step.ToolInput) > 0 {
				sb.WriteString(fmt.Sprintf("   - Input: %v\n", step.ToolInput))
			}
			sb.WriteString("\n")
		}
	}

	// 失败教训
	if len(result.FailureLessons) > 0 {
		sb.WriteString("## Lessons Learned\n\n")
		for i, lesson := range result.FailureLessons {
			sb.WriteString(fmt.Sprintf("%d. **%s** (iteration %d)\n", i+1, lesson.Type, lesson.Iteration))
			sb.WriteString(fmt.Sprintf("   - %s\n", lesson.Description))
			sb.WriteString(fmt.Sprintf("   - *Suggestion: %s*\n", lesson.Suggestion))
			sb.WriteString("\n")
		}
	}

	// 新知识
	if len(result.NewKnowledge) > 0 {
		sb.WriteString("## New Knowledge\n\n")
		for i, node := range result.NewKnowledge {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, node.Content))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// FormatJSON 格式化为 JSON 报告
func (f *ReactReplayFormatter) FormatJSON(result *ReactReplayResult) (string, error) {
	if result == nil {
		return "", nil
	}

	// 简单实现，实际应使用 json.Marshal
	return fmt.Sprintf(`{"session_id":"%s","confidence":%.2f,"success_paths":%d,"lessons":%d,"knowledge":%d}`,
		result.SessionID,
		result.Confidence,
		len(result.SuccessPath),
		len(result.FailureLessons),
		len(result.NewKnowledge),
	), nil
}
