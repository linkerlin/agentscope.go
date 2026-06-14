package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// ReactStepType ReAct 步骤类型
type ReactStepType string

const (
	StepReasoning   ReactStepType = "reasoning"
	StepActing      ReactStepType = "acting"
	StepObservation ReactStepType = "observation"
	StepFinal       ReactStepType = "final"
)

// ReactStep 单步 ReAct 记录
type ReactStep struct {
	ID          string                  `json:"id"`
	Iteration   int                     `json:"iteration"`
	Type        ReactStepType           `json:"type"`
	Timestamp   time.Time               `json:"timestamp"`
	Messages    []*message.Msg          `json:"messages"`
	MemoryNodes []*MemoryNode           `json:"memory_nodes,omitempty"`
	ToolCalls   []*message.ToolUseBlock `json:"tool_calls,omitempty"`
	Metadata    map[string]any          `json:"metadata,omitempty"`
}

// NewReactStep 创建 ReAct 步骤记录
func NewReactStep(iteration int, stepType ReactStepType, msgs []*message.Msg) *ReactStep {
	return &ReactStep{
		ID:        fmt.Sprintf("step-%d-%s-%d", iteration, stepType, time.Now().UnixNano()),
		Iteration: iteration,
		Type:      stepType,
		Timestamp: time.Now(),
		Messages:  cloneMsgSlice(msgs),
		Metadata:  make(map[string]any),
	}
}

// WithMemoryNodes 附加检索到的记忆节点
func (s *ReactStep) WithMemoryNodes(nodes []*MemoryNode) *ReactStep {
	if s == nil {
		return nil
	}
	s.MemoryNodes = append(s.MemoryNodes, nodes...)
	return s
}

// WithToolCalls 附加工具调用
func (s *ReactStep) WithToolCalls(calls []*message.ToolUseBlock) *ReactStep {
	if s == nil {
		return nil
	}
	s.ToolCalls = append(s.ToolCalls, calls...)
	return s
}

// WithMetadata 附加元数据
func (s *ReactStep) WithMetadata(key string, value any) *ReactStep {
	if s == nil || s.Metadata == nil {
		return s
	}
	s.Metadata[key] = value
	return s
}

// ReactStepStore 步级存储接口
type ReactStepStore interface {
	// Save 保存单步记录
	Save(ctx context.Context, step *ReactStep) error
	// GetByIteration 按迭代号查询步历史
	GetByIteration(ctx context.Context, iteration int) ([]*ReactStep, error)
	// GetByType 按类型查询步历史
	GetByType(ctx context.Context, stepType ReactStepType) ([]*ReactStep, error)
	// GetAll 获取完整 ReAct 循环的所有步
	GetAll(ctx context.Context) ([]*ReactStep, error)
	// Clear 清空存储
	Clear(ctx context.Context) error
}

// InMemoryStepStore 内存步级存储
type InMemoryStepStore struct {
	mu    sync.RWMutex
	steps []*ReactStep
}

// NewInMemoryStepStore 创建内存步级存储
func NewInMemoryStepStore() *InMemoryStepStore {
	return &InMemoryStepStore{
		steps: make([]*ReactStep, 0),
	}
}

// Save 保存单步记录
func (s *InMemoryStepStore) Save(ctx context.Context, step *ReactStep) error {
	_ = ctx
	if step == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
	return nil
}

// GetByIteration 按迭代号查询步历史
func (s *InMemoryStepStore) GetByIteration(ctx context.Context, iteration int) ([]*ReactStep, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ReactStep
	for _, step := range s.steps {
		if step.Iteration == iteration {
			result = append(result, step)
		}
	}
	return result, nil
}

// GetByType 按类型查询步历史
func (s *InMemoryStepStore) GetByType(ctx context.Context, stepType ReactStepType) ([]*ReactStep, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ReactStep
	for _, step := range s.steps {
		if step.Type == stepType {
			result = append(result, step)
		}
	}
	return result, nil
}

// GetAll 获取完整 ReAct 循环的所有步
func (s *InMemoryStepStore) GetAll(ctx context.Context) ([]*ReactStep, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ReactStep, len(s.steps))
	copy(result, s.steps)
	return result, nil
}

// Clear 清空存储
func (s *InMemoryStepStore) Clear(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = s.steps[:0]
	return nil
}

// Stats 返回存储统计
func (s *InMemoryStepStore) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]int{
		"total":       len(s.steps),
		"reasoning":   0,
		"acting":      0,
		"observation": 0,
		"final":       0,
	}
	for _, step := range s.steps {
		switch step.Type {
		case StepReasoning:
			stats["reasoning"]++
		case StepActing:
			stats["acting"]++
		case StepObservation:
			stats["observation"]++
		case StepFinal:
			stats["final"]++
		}
	}
	return stats
}

// ReactStepRecorder 步级记录器，用于在 ReAct 循环中记录每步
type ReactStepRecorder struct {
	store ReactStepStore
}

// NewReactStepRecorder 创建步级记录器
func NewReactStepRecorder(store ReactStepStore) *ReactStepRecorder {
	if store == nil {
		store = NewInMemoryStepStore()
	}
	return &ReactStepRecorder{store: store}
}

// RecordReasoning 记录 reasoning 步骤
func (r *ReactStepRecorder) RecordReasoning(ctx context.Context, iteration int, msgs []*message.Msg, memoryNodes []*MemoryNode) (*ReactStep, error) {
	step := NewReactStep(iteration, StepReasoning, msgs).
		WithMemoryNodes(memoryNodes)
	return step, r.store.Save(ctx, step)
}

// RecordActing 记录 acting 步骤
func (r *ReactStepRecorder) RecordActing(ctx context.Context, iteration int, toolCalls []*message.ToolUseBlock) (*ReactStep, error) {
	step := NewReactStep(iteration, StepActing, nil).
		WithToolCalls(toolCalls)
	return step, r.store.Save(ctx, step)
}

// RecordObservation 记录 observation 步骤
func (r *ReactStepRecorder) RecordObservation(ctx context.Context, iteration int, toolResults []*message.Msg) (*ReactStep, error) {
	step := NewReactStep(iteration, StepObservation, toolResults)
	return step, r.store.Save(ctx, step)
}

// RecordFinal 记录 final 步骤
func (r *ReactStepRecorder) RecordFinal(ctx context.Context, iteration int, finalMsg *message.Msg) (*ReactStep, error) {
	step := NewReactStep(iteration, StepFinal, []*message.Msg{finalMsg})
	return step, r.store.Save(ctx, step)
}

// GetStore 返回底层存储
func (r *ReactStepRecorder) GetStore() ReactStepStore {
	return r.store
}

// GetAllSteps 获取所有步骤
func (r *ReactStepRecorder) GetAllSteps(ctx context.Context) ([]*ReactStep, error) {
	return r.store.GetAll(ctx)
}

// ReactStepSequence ReAct 步序列（完整循环）
type ReactStepSequence struct {
	SessionID     string       `json:"session_id"`
	AgentName     string       `json:"agent_name"`
	StartTime     time.Time    `json:"start_time"`
	EndTime       *time.Time   `json:"end_time,omitempty"`
	Steps         []*ReactStep `json:"steps"`
	MaxIterations int          `json:"max_iterations"`
	FinalAnswer   string       `json:"final_answer,omitempty"`
	Success       bool         `json:"success"`
}

// BuildSequence 从存储构建完整序列
func BuildSequence(ctx context.Context, store ReactStepStore, sessionID, agentName string) (*ReactStepSequence, error) {
	steps, err := store.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	seq := &ReactStepSequence{
		SessionID: sessionID,
		AgentName: agentName,
		Steps:     steps,
	}

	if len(steps) > 0 {
		seq.StartTime = steps[0].Timestamp
		seq.MaxIterations = steps[len(steps)-1].Iteration + 1

		// 检查是否有 final 步骤
		for _, step := range steps {
			if step.Type == StepFinal {
				seq.Success = true
				seq.EndTime = &step.Timestamp
				if len(step.Messages) > 0 {
					seq.FinalAnswer = step.Messages[0].GetTextContent()
				}
				break
			}
		}
	}

	return seq, nil
}

// Summary 生成步序列摘要
func (seq *ReactStepSequence) Summary() string {
	if seq == nil {
		return ""
	}

	var sb string
	sb += fmt.Sprintf("ReAct Session: %s\n", seq.SessionID)
	sb += fmt.Sprintf("Agent: %s\n", seq.AgentName)
	sb += fmt.Sprintf("Steps: %d (max iterations: %d)\n", len(seq.Steps), seq.MaxIterations)
	sb += fmt.Sprintf("Success: %v\n", seq.Success)
	if seq.Success {
		sb += fmt.Sprintf("Final Answer: %s\n", seq.FinalAnswer)
	}
	if seq.EndTime != nil {
		duration := seq.EndTime.Sub(seq.StartTime)
		sb += fmt.Sprintf("Duration: %v\n", duration)
	}
	return sb
}
