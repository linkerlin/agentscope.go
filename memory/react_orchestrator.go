package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// ReactOrchestrator ReAct 记忆编排器
// 在现有 ReActAgent 基础上，增强记忆模块的编排能力
type ReactOrchestrator struct {
	Orchestrator    any              // 已有编排器（通过接口解耦，避免循环依赖）
	StepRecorder    *ReactStepRecorder   // 步级记录器
	MemoryStore     VectorStore          // 向量存储（用于检索注入）
	Config          ReactOrchestratorConfig  // 配置
	mu              sync.RWMutex
}

// ReactOrchestratorConfig ReAct 编排器配置
type ReactOrchestratorConfig struct {
	EnableMemoryInjection bool          // 启用记忆注入
	MaxInjectedMemories   int           // 每步最大注入记忆数
	MinMemoryScore        float64       // 注入记忆最低分数
	InjectionStrategy     InjectionStrategy // 注入策略
	TokenBudget           int           // 注入记忆 Token 预算
	EnableStepRecording   bool          // 启用步级记录
	EnableReplay          bool          // 启用复盘
}

// InjectionStrategy 记忆注入策略
type InjectionStrategy string

const (
	// InjectRecent 最近相关记忆（时间衰减）
	InjectRecent InjectionStrategy = "recent"
	// InjectTargeted 任务相关记忆（MemoryTarget 匹配）
	InjectTargeted InjectionStrategy = "targeted"
	// InjectPersonal 用户画像记忆
	InjectPersonal InjectionStrategy = "personal"
	// InjectHybrid 混合策略（加权组合）
	InjectHybrid InjectionStrategy = "hybrid"
)

// DefaultReactOrchestratorConfig 返回默认配置
func DefaultReactOrchestratorConfig() ReactOrchestratorConfig {
	return ReactOrchestratorConfig{
		EnableMemoryInjection: true,
		MaxInjectedMemories:   5,
		MinMemoryScore:        0.5,
		InjectionStrategy:     InjectHybrid,
		TokenBudget:           500,
		EnableStepRecording:   true,
		EnableReplay:          true,
	}
}

// NewReactOrchestrator 创建 ReAct 记忆编排器
func NewReactOrchestrator(
	orchestrator any,
	memoryStore VectorStore,
	config ReactOrchestratorConfig,
) *ReactOrchestrator {
	return &ReactOrchestrator{
		Orchestrator: orchestrator,
		StepRecorder: NewReactStepRecorder(nil),
		MemoryStore:  memoryStore,
		Config:       config,
	}
}

// InjectMemory 检索相关记忆注入到 reasoning 步骤
// 根据当前查询 + 历史上下文检索记忆，并格式化为系统消息注入
func (ro *ReactOrchestrator) InjectMemory(ctx context.Context, query string, history []*message.Msg, userName, taskName string) ([]*MemoryNode, *message.Msg, error) {
	if !ro.Config.EnableMemoryInjection || ro.MemoryStore == nil {
		return nil, nil, nil
	}

	ro.mu.RLock()
	config := ro.Config
	ro.mu.RUnlock()

	// 构建检索查询（当前查询 + 历史上下文关键词）
	searchQuery := ro.buildSearchQuery(query, history)

	// 根据策略检索记忆
	var nodes []*MemoryNode
	var err error

	switch config.InjectionStrategy {
	case InjectRecent:
		nodes, err = ro.searchRecent(ctx, searchQuery, config.MaxInjectedMemories)
	case InjectTargeted:
		nodes, err = ro.searchTargeted(ctx, searchQuery, userName, taskName, config.MaxInjectedMemories)
	case InjectPersonal:
		nodes, err = ro.searchPersonal(ctx, searchQuery, userName, config.MaxInjectedMemories)
	case InjectHybrid:
		fallthrough
	default:
		nodes, err = ro.searchHybrid(ctx, searchQuery, userName, taskName, config.MaxInjectedMemories)
	}

	if err != nil || len(nodes) == 0 {
		return nil, nil, err
	}

	// 过滤低分记忆
	nodes = ro.filterByScore(nodes, config.MinMemoryScore)
	if len(nodes) == 0 {
		return nil, nil, nil
	}

	// 限制数量
	if len(nodes) > config.MaxInjectedMemories {
		nodes = nodes[:config.MaxInjectedMemories]
	}

	// 格式化为注入消息
	injectMsg := ro.formatMemoryInjection(nodes)

	return nodes, injectMsg, nil
}

// buildSearchQuery 构建检索查询
func (ro *ReactOrchestrator) buildSearchQuery(query string, history []*message.Msg) string {
	// 简单实现：使用当前查询 + 最近用户消息的内容
	if query != "" {
		return query
	}

	// 从历史中提取最近用户消息
	for i := len(history) - 1; i >= 0; i-- {
		if history[i] != nil && history[i].Role == message.RoleUser {
			return history[i].GetTextContent()
		}
	}

	return ""
}

// searchRecent 检索最近相关记忆
func (ro *ReactOrchestrator) searchRecent(ctx context.Context, query string, topK int) ([]*MemoryNode, error) {
	return ro.MemoryStore.Search(ctx, query, RetrieveOptions{
		TopK:     topK,
		MinScore: 0.3,
	})
}

// searchTargeted 检索任务相关记忆
func (ro *ReactOrchestrator) searchTargeted(ctx context.Context, query, userName, taskName string, topK int) ([]*MemoryNode, error) {
	var targets []string
	if userName != "" {
		targets = append(targets, userName)
	}
	if taskName != "" {
		targets = append(targets, taskName)
	}

	return ro.MemoryStore.Search(ctx, query, RetrieveOptions{
		TopK:          topK,
		MinScore:      0.3,
		MemoryTargets: targets,
	})
}

// searchPersonal 检索用户画像记忆
func (ro *ReactOrchestrator) searchPersonal(ctx context.Context, query, userName string, topK int) ([]*MemoryNode, error) {
	if userName == "" {
		return nil, nil
	}

	return ro.MemoryStore.Search(ctx, query, RetrieveOptions{
		TopK:        topK,
		MinScore:    0.3,
		MemoryTypes: []MemoryType{MemoryTypePersonal},
		MemoryTargets: []string{userName},
	})
}

// searchHybrid 混合策略检索
func (ro *ReactOrchestrator) searchHybrid(ctx context.Context, query, userName, taskName string, topK int) ([]*MemoryNode, error) {
	// 并行检索多种类型
	var all []*MemoryNode
	var mu sync.Mutex

	// 1. 通用检索
	if nodes, err := ro.searchRecent(ctx, query, topK); err == nil {
		mu.Lock()
		all = append(all, nodes...)
		mu.Unlock()
	}

	// 2. 任务相关
	if taskName != "" {
		if nodes, err := ro.searchTargeted(ctx, query, userName, taskName, topK/2); err == nil {
			mu.Lock()
			all = append(all, nodes...)
			mu.Unlock()
		}
	}

	// 3. 用户画像
	if userName != "" {
		if nodes, err := ro.searchPersonal(ctx, query, userName, topK/2); err == nil {
			mu.Lock()
			all = append(all, nodes...)
			mu.Unlock()
		}
	}

	// 去重并按分数排序
	return ro.dedupAndSort(all), nil
}

// filterByScore 按分数过滤记忆
func (ro *ReactOrchestrator) filterByScore(nodes []*MemoryNode, minScore float64) []*MemoryNode {
	var result []*MemoryNode
	for _, n := range nodes {
		if n != nil && n.Score >= minScore {
			result = append(result, n)
		}
	}
	return result
}

// dedupAndSort 去重并按分数排序
func (ro *ReactOrchestrator) dedupAndSort(nodes []*MemoryNode) []*MemoryNode {
	seen := make(map[string]bool)
	var unique []*MemoryNode

	for _, n := range nodes {
		if n == nil || seen[n.MemoryID] {
			continue
		}
		seen[n.MemoryID] = true
		unique = append(unique, n)
	}

	// 按分数降序排序
	for i := 0; i < len(unique)-1; i++ {
		for j := i + 1; j < len(unique); j++ {
			if unique[j].Score > unique[i].Score {
				unique[i], unique[j] = unique[j], unique[i]
			}
		}
	}

	return unique
}

// formatMemoryInjection 将记忆格式化为注入消息
func (ro *ReactOrchestrator) formatMemoryInjection(nodes []*MemoryNode) *message.Msg {
	if len(nodes) == 0 {
		return nil
	}

	var sb string
	sb += "## Relevant Memories\n\n"
	for i, n := range nodes {
		sb += fmt.Sprintf("%d. [%s] %s\n", i+1, n.MemoryType, n.Content)
		if n.WhenToUse != "" {
			sb += fmt.Sprintf("   *When to use: %s*\n", n.WhenToUse)
		}
		sb += "\n"
	}

	return message.NewMsg().
		Role(message.RoleSystem).
		TextContent(sb).
		Build()
}

// RecordReActStep 记录 ReAct 单步
func (ro *ReactOrchestrator) RecordReActStep(ctx context.Context, iteration int, stepType ReactStepType, msgs []*message.Msg, memoryNodes []*MemoryNode) (*ReactStep, error) {
	if !ro.Config.EnableStepRecording {
		return nil, nil
	}

	switch stepType {
	case StepReasoning:
		return ro.StepRecorder.RecordReasoning(ctx, iteration, msgs, memoryNodes)
	case StepActing:
		var calls []*message.ToolUseBlock
		for _, m := range msgs {
			if m != nil {
				calls = append(calls, m.GetToolUseCalls()...)
			}
		}
		return ro.StepRecorder.RecordActing(ctx, iteration, calls)
	case StepObservation:
		return ro.StepRecorder.RecordObservation(ctx, iteration, msgs)
	case StepFinal:
		if len(msgs) > 0 {
			return ro.StepRecorder.RecordFinal(ctx, iteration, msgs[0])
		}
	}

	return nil, nil
}

// GetStepHistory 获取步历史
func (ro *ReactOrchestrator) GetStepHistory(ctx context.Context) ([]*ReactStep, error) {
	return ro.StepRecorder.GetAllSteps(ctx)
}

// GetStepSequence 获取完整步序列
func (ro *ReactOrchestrator) GetStepSequence(ctx context.Context, sessionID, agentName string) (*ReactStepSequence, error) {
	return BuildSequence(ctx, ro.StepRecorder.GetStore(), sessionID, agentName)
}

// ClearSteps 清空步记录
func (ro *ReactOrchestrator) ClearSteps(ctx context.Context) error {
	return ro.StepRecorder.GetStore().Clear(ctx)
}

// ReactOrchestratorStats ReAct 编排器统计
type ReactOrchestratorStats struct {
	TotalSteps        int       `json:"total_steps"`
	InjectedMemories  int       `json:"injected_memories"`
	ReasoningSteps    int       `json:"reasoning_steps"`
	ActingSteps       int       `json:"acting_steps"`
	ObservationSteps  int       `json:"observation_steps"`
	FinalSteps        int       `json:"final_steps"`
	AvgInjectionCount float64   `json:"avg_injection_count"`
	LastUpdated       time.Time `json:"last_updated"`
}

// Stats 返回编排器统计
func (ro *ReactOrchestrator) Stats() *ReactOrchestratorStats {
	ro.mu.RLock()
	defer ro.mu.RUnlock()

	stepStats := ro.StepRecorder.GetStore().(*InMemoryStepStore).Stats()

	return &ReactOrchestratorStats{
		TotalSteps:       stepStats["total"],
		ReasoningSteps:   stepStats["reasoning"],
		ActingSteps:      stepStats["acting"],
		ObservationSteps: stepStats["observation"],
		FinalSteps:       stepStats["final"],
		LastUpdated:      time.Now(),
	}
}

// WithStepRecorder 设置自定义步级记录器
func (ro *ReactOrchestrator) WithStepRecorder(recorder *ReactStepRecorder) *ReactOrchestrator {
	ro.StepRecorder = recorder
	return ro
}

// WithConfig 更新配置
func (ro *ReactOrchestrator) WithConfig(config ReactOrchestratorConfig) *ReactOrchestrator {
	ro.mu.Lock()
	defer ro.mu.Unlock()
	ro.Config = config
	return ro
}
