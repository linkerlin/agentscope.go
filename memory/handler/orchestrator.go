package handler

import (
	"context"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// MemoryOrchestrator 记忆编排器，协调 Summarize 与 Retrieve 完整流程
type MemoryOrchestrator struct {
	PersonalSum   *memory.PersonalSummarizer
	ProceduralSum *memory.ProceduralSummarizer
	ToolSum       *memory.ToolSummarizer

	MemoryTool  *MemoryHandler
	ProfileTool *ProfileHandler
	HistoryTool *HistoryHandler
	Dedup       *memory.MemoryDeduplicator

	Config memory.OrchestratorConfig

	toolMu       sync.Mutex
	toolResults  map[string][]memory.ToolCallResult
}

// NewMemoryOrchestrator 创建编排器
func NewMemoryOrchestrator(
	personalSum *memory.PersonalSummarizer,
	proceduralSum *memory.ProceduralSummarizer,
	toolSum *memory.ToolSummarizer,
	memTool *MemoryHandler,
	profileTool *ProfileHandler,
	historyTool *HistoryHandler,
	dedup *memory.MemoryDeduplicator,
) *MemoryOrchestrator {
	return &MemoryOrchestrator{
		PersonalSum:   personalSum,
		ProceduralSum: proceduralSum,
		ToolSum:       toolSum,
		MemoryTool:    memTool,
		ProfileTool:   profileTool,
		HistoryTool:   historyTool,
		Dedup:         dedup,
		Config:        memory.DefaultOrchestratorConfig(),
	}
}

// Summarize 端到端记忆提取与持久化
func (o *MemoryOrchestrator) Summarize(ctx context.Context, msgs []*message.Msg, userName, taskName, toolName string) (*memory.SummarizeResult, error) {
	res := &memory.SummarizeResult{
		UpdatedProfiles: make(map[string]map[string]any),
	}

	// 1) 添加历史记录
	if o.Config.EnableHistory && o.HistoryTool != nil {
		target := firstNonEmpty(userName, taskName, toolName)
		if target != "" {
			histNode, err := o.HistoryTool.AddHistory(ctx, msgs, target, "")
			if err == nil && histNode != nil {
				res.AddedHistory = histNode
			}
		}
	}

	// 2) Personal Memory
	if o.Config.EnablePersonal && userName != "" && o.PersonalSum != nil {
		nodes, profile, err := o.summarizePersonal(ctx, msgs, userName)
		if err == nil {
			res.PersonalMemories = nodes
			if profile != nil {
				res.UpdatedProfiles[userName] = profile
			}
		}
	}

	// 3) Procedural Memory
	if o.Config.EnableProcedural && taskName != "" && o.ProceduralSum != nil {
		nodes, err := o.summarizeProcedural(ctx, msgs, taskName)
		if err == nil {
			res.ProceduralMemories = nodes
		}
	}

	// 4) Tool Memory
	if o.Config.EnableTool && toolName != "" && o.ToolSum != nil {
		if err := o.SummarizeToolUsage(ctx, toolName); err == nil {
			res.ToolMemories = o.flushToolResults(toolName)
		}
	}

	return res, nil
}

// Retrieve 统一检索入口，按类型分发后合并去重
func (o *MemoryOrchestrator) Retrieve(ctx context.Context, query string, userName, taskName, toolName string, opts memory.RetrieveOptions) ([]*memory.MemoryNode, error) {
	var all []*memory.MemoryNode

	if userName != "" && o.MemoryTool != nil {
		nodes, _ := o.MemoryTool.RetrieveMemory(ctx, query, memory.RetrieveOptions{
			TopK:          opts.TopK,
			MinScore:      opts.MinScore,
			MemoryTypes:   []memory.MemoryType{memory.MemoryTypePersonal},
			MemoryTargets: []string{userName},
		})
		all = append(all, nodes...)
	}

	if taskName != "" && o.MemoryTool != nil {
		nodes, _ := o.MemoryTool.RetrieveMemory(ctx, query, memory.RetrieveOptions{
			TopK:          opts.TopK,
			MinScore:      opts.MinScore,
			MemoryTypes:   []memory.MemoryType{memory.MemoryTypeProcedural},
			MemoryTargets: []string{taskName},
		})
		all = append(all, nodes...)
	}

	if toolName != "" && o.MemoryTool != nil {
		nodes, _ := o.MemoryTool.RetrieveMemory(ctx, query, memory.RetrieveOptions{
			TopK:          opts.TopK,
			MinScore:      opts.MinScore,
			MemoryTypes:   []memory.MemoryType{memory.MemoryTypeTool},
			MemoryTargets: []string{toolName},
		})
		all = append(all, nodes...)
	}

	if o.Dedup != nil {
		all, _, _ = o.Dedup.Deduplicate(ctx, all)
	}
	return all, nil
}

// summarizePersonal 个人记忆提取与持久化；返回 (记忆列表, 更新后的profile, error)
func (o *MemoryOrchestrator) summarizePersonal(ctx context.Context, msgs []*message.Msg, userName string) ([]*memory.MemoryNode, map[string]any, error) {
	observations, err := o.PersonalSum.ExtractObservations(ctx, msgs, userName)
	if err != nil || len(observations) == 0 {
		return nil, nil, err
	}

	insights, _ := o.PersonalSum.ExtractInsights(ctx, observations, userName)
	all := append(observations, insights...)

	if o.Dedup != nil {
		all, _, _ = o.Dedup.Deduplicate(ctx, all)
	}

	var stored []*memory.MemoryNode
	for _, node := range all {
		node.MemoryTarget = userName
		node.MemoryType = memory.MemoryTypePersonal
		if err := o.writeMemoryNode(ctx, node); err != nil {
			continue
		}
		stored = append(stored, node)
	}

	// 更新 Profile
	var profile map[string]any
	if o.Config.EnableProfile && o.ProfileTool != nil && len(stored) > 0 {
		updates := make(map[string]any)
		for _, n := range stored {
			if keywords, ok := n.Metadata["keywords"].(string); ok && keywords != "" {
				updates[keywords] = n.Content
			}
			if subject, ok := n.Metadata["insight_subject"].(string); ok && subject != "" {
				updates[subject] = n.Content
			}
		}
		if len(updates) > 0 {
			_ = o.ProfileTool.UpdateProfile(ctx, userName, updates)
			profile, _ = o.ProfileTool.ReadAllProfiles(ctx, userName)
		}
	}

	return stored, profile, nil
}

// summarizeProcedural 任务经验提取与持久化
func (o *MemoryOrchestrator) summarizeProcedural(ctx context.Context, msgs []*message.Msg, taskName string) ([]*memory.MemoryNode, error) {
	traj := memory.Trajectory{
		Messages: msgs,
		Score:    1.0, // 默认视为成功轨迹，后续可由调用方传入评分
		TaskName: taskName,
	}
	nodes, err := o.ProceduralSum.ExtractFromSingleTrajectory(ctx, traj)
	if err != nil || len(nodes) == 0 {
		return nil, err
	}

	nodes = o.ProceduralSum.DeduplicateMemories(nodes)

	var stored []*memory.MemoryNode
	for _, node := range nodes {
		node.MemoryTarget = taskName
		node.MemoryType = memory.MemoryTypeProcedural
		if err := o.writeMemoryNode(ctx, node); err != nil {
			continue
		}
		stored = append(stored, node)
	}
	return stored, nil
}

// writeMemoryNode 写入策略：高相似则更新，否则新增
func (o *MemoryOrchestrator) writeMemoryNode(ctx context.Context, node *memory.MemoryNode) error {
	if o.MemoryTool == nil {
		return nil
	}
	similar, _ := o.MemoryTool.AddDraftAndRetrieveSimilar(ctx, node, o.Config.RetrieveTopK)

	var updateTarget *memory.MemoryNode
	for _, s := range similar {
		if s.Score >= 0.9 {
			updateTarget = s
			break
		}
	}

	node.TimeModified = time.Now()
	if updateTarget != nil {
		updateTarget.Content = node.Content
		updateTarget.WhenToUse = node.WhenToUse
		updateTarget.Metadata = node.Metadata
		updateTarget.TimeModified = node.TimeModified
		return o.MemoryTool.UpdateMemory(ctx, updateTarget)
	}
	return o.MemoryTool.AddMemory(ctx, node)
}

// AddToolCallResult 收集工具调用结果
func (o *MemoryOrchestrator) AddToolCallResult(result memory.ToolCallResult) error {
	o.toolMu.Lock()
	defer o.toolMu.Unlock()
	if o.toolResults == nil {
		o.toolResults = make(map[string][]memory.ToolCallResult)
	}
	o.toolResults[result.ToolName] = append(o.toolResults[result.ToolName], result)
	return nil
}

// SummarizeToolUsage 对指定工具的调用记录进行总结并持久化
func (o *MemoryOrchestrator) SummarizeToolUsage(ctx context.Context, toolName string) error {
	if o.ToolSum == nil || o.MemoryTool == nil {
		return nil
	}
	o.toolMu.Lock()
	results := o.toolResults[toolName]
	o.toolMu.Unlock()
	if len(results) == 0 {
		return nil
	}
	node, err := o.ToolSum.SummarizeToolUsage(ctx, toolName, results)
	if err != nil || node == nil {
		return err
	}
	return o.MemoryTool.AddMemory(ctx, node)
}

func (o *MemoryOrchestrator) flushToolResults(toolName string) []*memory.MemoryNode {
	o.toolMu.Lock()
	defer o.toolMu.Unlock()
	delete(o.toolResults, toolName)
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

var _ memory.Orchestrator = (*MemoryOrchestrator)(nil)
