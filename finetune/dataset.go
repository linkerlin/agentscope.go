package finetune

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// TrainingSample 表示一条 SFT/RL 训练样本。
type TrainingSample struct {
	ID        string          `json:"id"`
	Messages  []*message.Msg  `json:"messages"`       // 对话历史
	ToolsUsed []ToolCall      `json:"tools_used"`     // 工具调用记录
	Reward    float64         `json:"reward"`         // 奖励分数（RL 用）
	Feedback  string          `json:"feedback"`       // 人工反馈
	Source    string          `json:"source"`         // 来源：agent_run / human / auto_eval
	CreatedAt time.Time       `json:"created_at"`
	Metadata  map[string]any  `json:"metadata"`
}

// ToolCall 记录工具调用详情。
type ToolCall struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
	Output   string         `json:"output"`
	Success  bool           `json:"success"`
	Duration int64          `json:"duration_ms"`
}

// TrainingDataset 训练数据集管理器。
type TrainingDataset struct {
	samples []*TrainingSample
}

// NewTrainingDataset 创建训练数据集。
func NewTrainingDataset() *TrainingDataset {
	return &TrainingDataset{samples: make([]*TrainingSample, 0)}
}

// AddSample 添加训练样本。
func (d *TrainingDataset) AddSample(sample *TrainingSample) {
	d.samples = append(d.samples, sample)
}

// ExportSFT 导出 SFT（监督微调）格式的训练数据（OpenAI messages 格式）。
func (d *TrainingDataset) ExportSFT() []map[string]any {
	result := make([]map[string]any, 0, len(d.samples))
	for _, s := range d.samples {
		messages := make([]map[string]string, 0, len(s.Messages))
		for _, m := range s.Messages {
			messages = append(messages, map[string]string{
				"role":    string(m.Role),
				"content": m.GetTextContent(),
			})
		}
		result = append(result, map[string]any{
			"messages": messages,
			"metadata": map[string]any{
				"source":   s.Source,
				"reward":   s.Reward,
				"feedback": s.Feedback,
			},
		})
	}
	return result
}

// ExportRL 导出 RL（强化学习）格式的训练数据（偏好对）。
func (d *TrainingDataset) ExportRL() []map[string]any {
	result := make([]map[string]any, 0)
	// 按任务分组，构建 chosen/rejected 对
	groups := d.groupByTask()
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		// 找到最高奖励和最低奖励的样本
		best, worst := group[0], group[0]
		for _, s := range group {
			if s.Reward > best.Reward {
				best = s
			}
			if s.Reward < worst.Reward {
				worst = s
			}
		}
		if best.Reward <= worst.Reward {
			continue
		}
		result = append(result, map[string]any{
			"prompt":   buildPrompt(best.Messages),
			"chosen":   best.Messages[len(best.Messages)-1].GetTextContent(),
			"rejected": worst.Messages[len(worst.Messages)-1].GetTextContent(),
			"reward_diff": best.Reward - worst.Reward,
		})
	}
	return result
}

// ExportJSONL 导出为 JSONL 格式（用于直接训练）。
func (d *TrainingDataset) ExportJSONL() ([]byte, error) {
	var buf []byte
	for _, s := range d.samples {
		line, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return buf, nil
}

// groupByTask 按任务分组样本。
func (d *TrainingDataset) groupByTask() map[string][]*TrainingSample {
	groups := make(map[string][]*TrainingSample)
	for _, s := range d.samples {
		task := s.Metadata["task_id"]
		if task == nil {
			task = "default"
		}
		key := fmt.Sprintf("%v", task)
		groups[key] = append(groups[key], s)
	}
	return groups
}

func buildPrompt(msgs []*message.Msg) string {
	if len(msgs) == 0 {
		return ""
	}
	// 最后一条 user 消息作为 prompt
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.RoleUser {
			return msgs[i].GetTextContent()
		}
	}
	return msgs[0].GetTextContent()
}

// EventCollector 从 Agent 事件流中收集训练数据。
type EventCollector struct {
	dataset *TrainingDataset
	current *TrainingSample
}

// NewEventCollector 创建事件收集器。
func NewEventCollector(dataset *TrainingDataset) *EventCollector {
	return &EventCollector{dataset: dataset}
}

// Collect 从事件流中收集训练数据。
func (c *EventCollector) Collect(ctx context.Context, ch <-chan event.AgentEvent) {
	c.current = &TrainingSample{
		ID:        generateID("sample"),
		Messages:  make([]*message.Msg, 0),
		ToolsUsed: make([]ToolCall, 0),
		Source:    "agent_run",
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}

	for ev := range ch {
		switch e := ev.(type) {
		case *event.TextBlockDeltaEvent:
			// 累积文本回复
		case *event.ToolCallStartEvent:
			c.current.ToolsUsed = append(c.current.ToolsUsed, ToolCall{
				ToolName: e.ToolName,
			})
		case *event.ToolResultTextDeltaEvent:
			if len(c.current.ToolsUsed) > 0 {
				last := &c.current.ToolsUsed[len(c.current.ToolsUsed)-1]
				last.Output += e.Delta
			}
		case *event.ReplyEndEvent:
			c.dataset.AddSample(c.current)
			c.current = nil
		}
	}
}

// AutoEvaluator 自动评估 Agent 回复质量，生成奖励分数。
type AutoEvaluator struct {
	// 评估维度权重
	weights map[string]float64
}

// NewAutoEvaluator 创建自动评估器。
func NewAutoEvaluator() *AutoEvaluator {
	return &AutoEvaluator{
		weights: map[string]float64{
			"correctness": 0.4,
			"helpfulness": 0.3,
			"conciseness": 0.2,
			"safety":      0.1,
		},
	}
}

// Evaluate 评估一条回复的质量。
func (e *AutoEvaluator) Evaluate(reply *message.Msg, expected string) float64 {
	score := 0.0

	// 1. 正确性：与期望输出的相似度
	if expected != "" {
		score += e.weights["correctness"] * cosineSimilarityText(reply.GetTextContent(), expected)
	}

	// 2. 有用性：回复长度适中（100-500 字最佳）
	content := reply.GetTextContent()
	length := len(content)
	if length >= 50 && length <= 1000 {
		score += e.weights["helpfulness"]
	} else if length > 0 {
		score += e.weights["helpfulness"] * 0.5
	}

	// 3. 简洁性：避免重复和冗余
	if length > 0 {
		score += e.weights["conciseness"] * (1.0 - min(1.0, float64(length-200)/1000.0))
	}

	// 4. 安全性：检查有害内容（简化版）
	if !containsHarmful(content) {
		score += e.weights["safety"]
	}

	return score
}

func cosineSimilarityText(a, b string) float64 {
	// 简化版：基于共同词的比例
	wordsA := tokenize(a)
	wordsB := tokenize(b)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}
	common := 0
	for w := range wordsA {
		if wordsB[w] {
			common++
		}
	}
	return float64(common) / float64(len(wordsA)+len(wordsB)-common)
}

func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	// 简化分词：按空格和标点分割
	start := 0
	for i, c := range text {
		if c == ' ' || c == '.' || c == ',' || c == '!' || c == '?' {
			if i > start {
				words[text[start:i]] = true
			}
			start = i + 1
		}
	}
	if start < len(text) {
		words[text[start:]] = true
	}
	return words
}

func containsHarmful(text string) bool {
	// 简化版：检查常见有害关键词
	harmful := []string{"hack", "exploit", "attack", "malware", "virus"}
	lower := text
	for _, h := range harmful {
		if contains(lower, h) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
