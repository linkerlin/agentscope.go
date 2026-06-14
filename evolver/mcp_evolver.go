package evolver

import (
	"context"
	"encoding/json"
	"fmt"
)

// MCP tool name constants for the evolver namespace.
const (
	toolListGenes     = "evolver__list_genes"
	toolUpsertGene    = "evolver__upsert_gene"
	toolDeleteGene    = "evolver__delete_gene"
	toolListCapsules  = "evolver__list_capsules"
	toolRun           = "evolver__run"
	toolReflect       = "evolver__reflect"
	toolSolidify      = "evolver__solidify"
	toolRemember      = "evolver__remember"
	toolRecall        = "evolver__recall"
	toolMeetingStart  = "evolver__meeting_start"
	toolMeetingStatus = "evolver__meeting_status"
	toolFetchTasks    = "evolver__fetch_tasks"
	toolClaimTask     = "evolver__claim_task"
	toolCompleteTask  = "evolver__complete_task"
	toolStats         = "evolver__stats"
	toolSafetyStatus  = "evolver__safety_status"
)

// MCPEvolver 通过 MCP 协议连接真实的 Evolver 后端。
// 它将 Evolver 接口映射为 MCP 工具调用，实现 GEP 自演化的生产级集成。
type MCPEvolver struct {
	// mcpCaller 是调用 MCP 工具的函数签名
	// 实际项目中可以替换为真实的 MCP 客户端
	mcpCaller func(ctx context.Context, toolName string, args map[string]any) (map[string]any, error)
}

// NewMCPEvolver 创建 MCP 桥接的 Evolver 客户端。
// mcpCaller 是一个函数，接收工具名和参数，返回结果。
func NewMCPEvolver(mcpCaller func(ctx context.Context, toolName string, args map[string]any) (map[string]any, error)) *MCPEvolver {
	return &MCPEvolver{mcpCaller: mcpCaller}
}

// ListGenes 通过 MCP 工具 evolver__list_genes 获取基因列表。
func (m *MCPEvolver) ListGenes(ctx context.Context, category string) ([]Gene, error) {
	args := map[string]any{}
	if category != "" {
		args["category"] = category
	}
	resp, err := m.mcpCaller(ctx, toolListGenes, args)
	if err != nil {
		return nil, fmt.Errorf("mcp list_genes: %w", err)
	}
	data, err := json.Marshal(resp["genes"])
	if err != nil {
		return nil, err
	}
	var genes []Gene
	if err := json.Unmarshal(data, &genes); err != nil {
		return nil, err
	}
	return genes, nil
}

// UpsertGene 通过 MCP 工具 evolver__upsert_gene 创建或更新基因。
func (m *MCPEvolver) UpsertGene(ctx context.Context, gene Gene) error {
	args := map[string]any{"gene": gene}
	_, err := m.mcpCaller(ctx, toolUpsertGene, args)
	if err != nil {
		return fmt.Errorf("mcp upsert_gene: %w", err)
	}
	return nil
}

// DeleteGene 通过 MCP 工具 evolver__delete_gene 删除基因。
func (m *MCPEvolver) DeleteGene(ctx context.Context, geneID string) error {
	args := map[string]any{"gene_id": geneID}
	_, err := m.mcpCaller(ctx, toolDeleteGene, args)
	if err != nil {
		return fmt.Errorf("mcp delete_gene: %w", err)
	}
	return nil
}

// ListCapsules 通过 MCP 工具 evolver__list_capsules 获取胶囊列表。
func (m *MCPEvolver) ListCapsules(ctx context.Context, limit int) ([]Capsule, error) {
	args := map[string]any{"limit": limit}
	resp, err := m.mcpCaller(ctx, toolListCapsules, args)
	if err != nil {
		return nil, fmt.Errorf("mcp list_capsules: %w", err)
	}
	data, err := json.Marshal(resp["capsules"])
	if err != nil {
		return nil, err
	}
	var capsules []Capsule
	if err := json.Unmarshal(data, &capsules); err != nil {
		return nil, err
	}
	return capsules, nil
}

// Run 通过 MCP 工具 evolver__run 执行 GEP 运行。
func (m *MCPEvolver) Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	args := map[string]any{
		"context":                cfg.Context,
		"strategy":               cfg.Strategy,
		"drift_enabled":          cfg.DriftEnabled,
		"exploration_rate":       cfg.ExplorationRate,
		"cycle_id":               cfg.CycleID,
		"selector_mode":          cfg.SelectorMode,
		"use_hierarchical_bayes": cfg.UseHierarchicalBayes,
	}
	resp, err := m.mcpCaller(ctx, toolRun, args)
	if err != nil {
		return nil, fmt.Errorf("mcp run: %w", err)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var result RunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Reflect 通过 MCP 工具 evolver__reflect 执行风险分析。
func (m *MCPEvolver) Reflect(ctx context.Context, req ReflectRequest) (*ReflectResult, error) {
	args := map[string]any{
		"context":          req.Context,
		"gene":             req.Gene,
		"signals":          req.Signals,
		"blast_radius":     req.BlastRadius,
		"proposed_changes": req.ProposedChanges,
		"modified_files":   req.ModifiedFiles,
	}
	resp, err := m.mcpCaller(ctx, toolReflect, args)
	if err != nil {
		return nil, fmt.Errorf("mcp reflect: %w", err)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var result ReflectResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Solidify 通过 MCP 工具 evolver__solidify 固化演化结果。
func (m *MCPEvolver) Solidify(ctx context.Context, req SolidifyRequest) (*SolidifyResult, error) {
	args := map[string]any{
		"intent":                    req.Intent,
		"summary":                   req.Summary,
		"signals":                   req.Signals,
		"gene":                      req.Gene,
		"capsule":                   req.Capsule,
		"blast_radius":              req.BlastRadius,
		"modified_files":            req.ModifiedFiles,
		"gep_output":                req.GEPOutput,
		"dry_run":                   req.DryRun,
		"decision_source":           req.DecisionSource,
		"primary_cause":             req.PrimaryCause,
		"contributing_factors":      req.ContributingFactors,
		"human_intervention":        req.HumanIntervention,
		"manual_intervention_count": req.ManualInterventionCount,
		"selector_mode":             req.SelectorMode,
		"run_id":                    req.RunID,
		"reused_asset_id":           req.ReusedAssetID,
		"source_type":               req.SourceType,
	}
	resp, err := m.mcpCaller(ctx, toolSolidify, args)
	if err != nil {
		return nil, fmt.Errorf("mcp solidify: %w", err)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var result SolidifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Remember 通过 MCP 工具 evolver__remember 记录演化记忆。
func (m *MCPEvolver) Remember(ctx context.Context, req RememberRequest) error {
	args := map[string]any{
		"text":       req.Text,
		"type":       req.Type,
		"id":         req.ID,
		"importance": req.Importance,
		"category":   req.Category,
		"scope":      req.Scope,
		"metadata":   req.Metadata,
	}
	_, err := m.mcpCaller(ctx, toolRemember, args)
	if err != nil {
		return fmt.Errorf("mcp remember: %w", err)
	}
	return nil
}

// Recall 通过 MCP 工具 evolver__recall 召回演化记忆。
func (m *MCPEvolver) Recall(ctx context.Context, req RecallRequest) ([]MemoryHit, error) {
	args := map[string]any{
		"query":     req.Query,
		"limit":     req.Limit,
		"scope":     req.Scope,
		"category":  req.Category,
		"min_score": req.MinScore,
	}
	resp, err := m.mcpCaller(ctx, toolRecall, args)
	if err != nil {
		return nil, fmt.Errorf("mcp recall: %w", err)
	}
	data, err := json.Marshal(resp["hits"])
	if err != nil {
		return nil, err
	}
	var hits []MemoryHit
	if err := json.Unmarshal(data, &hits); err != nil {
		return nil, err
	}
	return hits, nil
}

// MeetingStart 通过 MCP 工具 evolver__meeting_start 启动演化会议。
func (m *MCPEvolver) MeetingStart(ctx context.Context, req MeetingStartRequest) (*Meeting, error) {
	args := map[string]any{
		"type":    req.Type,
		"task":    req.Task,
		"context": req.Context,
		"signals": req.Signals,
		"options": req.Options,
	}
	resp, err := m.mcpCaller(ctx, toolMeetingStart, args)
	if err != nil {
		return nil, fmt.Errorf("mcp meeting_start: %w", err)
	}
	data, err := json.Marshal(resp["meeting"])
	if err != nil {
		return nil, err
	}
	var meeting Meeting
	if err := json.Unmarshal(data, &meeting); err != nil {
		return nil, err
	}
	return &meeting, nil
}

// MeetingStatus 通过 MCP 工具 evolver__meeting_status 获取会议状态。
func (m *MCPEvolver) MeetingStatus(ctx context.Context, meetingID string) (*MeetingStatus, error) {
	args := map[string]any{"meeting_id": meetingID}
	resp, err := m.mcpCaller(ctx, toolMeetingStatus, args)
	if err != nil {
		return nil, fmt.Errorf("mcp meeting_status: %w", err)
	}
	data, err := json.Marshal(resp["status"])
	if err != nil {
		return nil, err
	}
	var status MeetingStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// FetchTasks 通过 MCP 工具 evolver__fetch_tasks 获取 ATP 任务。
func (m *MCPEvolver) FetchTasks(ctx context.Context, questions []any) ([]Task, error) {
	args := map[string]any{"questions": questions}
	resp, err := m.mcpCaller(ctx, toolFetchTasks, args)
	if err != nil {
		return nil, fmt.Errorf("mcp fetch_tasks: %w", err)
	}
	data, err := json.Marshal(resp["tasks"])
	if err != nil {
		return nil, err
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ClaimTask 通过 MCP 工具 evolver__claim_task 认领任务。
func (m *MCPEvolver) ClaimTask(ctx context.Context, taskID string) error {
	args := map[string]any{"task_id": taskID}
	_, err := m.mcpCaller(ctx, toolClaimTask, args)
	if err != nil {
		return fmt.Errorf("mcp claim_task: %w", err)
	}
	return nil
}

// CompleteTask 通过 MCP 工具 evolver__complete_task 完成任务。
func (m *MCPEvolver) CompleteTask(ctx context.Context, taskID, assetID string) error {
	args := map[string]any{
		"task_id":  taskID,
		"asset_id": assetID,
	}
	_, err := m.mcpCaller(ctx, toolCompleteTask, args)
	if err != nil {
		return fmt.Errorf("mcp complete_task: %w", err)
	}
	return nil
}

// Stats 通过 MCP 工具 evolver__stats 获取统计信息。
func (m *MCPEvolver) Stats(ctx context.Context) (map[string]any, error) {
	resp, err := m.mcpCaller(ctx, toolStats, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("mcp stats: %w", err)
	}
	return resp, nil
}

// SafetyStatus 通过 MCP 工具 evolver__safety_status 获取安全状态。
func (m *MCPEvolver) SafetyStatus(ctx context.Context) (map[string]any, error) {
	resp, err := m.mcpCaller(ctx, toolSafetyStatus, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("mcp safety_status: %w", err)
	}
	return resp, nil
}

// Verify interface compliance
var _ Evolver = (*MCPEvolver)(nil)
