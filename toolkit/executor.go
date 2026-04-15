package toolkit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/retry"
)

// ToolCall 单次工具调用描述（对齐模型 tool_use id）
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult 单次执行结果
type ToolResult struct {
	ID     string
	Name   string
	Result any
	Err    error
}

// ExecutionConfig 执行策略
type ExecutionConfig struct {
	Timeout     time.Duration
	MaxRetries  int
	Parallel    bool
	MaxParallel int // <=0 表示不限制（仍受 Parallel 控制）
}

// DefaultExecutionConfig 保守默认：顺序执行、不重试、无额外超时（由 context 控制）
func DefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{MaxRetries: 1}
}

// ToolExecutor 对 Registry 执行 ToolCall 列表
type ToolExecutor struct {
	cfg ExecutionConfig
}

// NewToolExecutor 创建执行器
func NewToolExecutor(cfg ExecutionConfig) *ToolExecutor {
	if cfg.MaxRetries < 1 {
		cfg.MaxRetries = 1
	}
	return &ToolExecutor{cfg: cfg}
}

// Execute 按配置顺序或并行执行多个调用
func (e *ToolExecutor) Execute(ctx context.Context, reg *Registry, calls []ToolCall) ([]ToolResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	if !e.cfg.Parallel || len(calls) == 1 {
		return e.executeSequential(ctx, reg, calls)
	}
	return e.executeParallel(ctx, reg, calls)
}

func (e *ToolExecutor) executeSequential(ctx context.Context, reg *Registry, calls []ToolCall) ([]ToolResult, error) {
	out := make([]ToolResult, 0, len(calls))
	for _, c := range calls {
		r := e.executeOne(ctx, reg, c)
		out = append(out, r)
	}
	return out, nil
}

func (e *ToolExecutor) executeParallel(ctx context.Context, reg *Registry, calls []ToolCall) ([]ToolResult, error) {
	maxP := e.cfg.MaxParallel
	if maxP <= 0 {
		maxP = len(calls)
	}
	sem := make(chan struct{}, maxP)
	var wg sync.WaitGroup
	results := make([]ToolResult, len(calls))
	for i := range calls {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = e.executeOne(ctx, reg, calls[i])
		}()
	}
	wg.Wait()
	return results, nil
}

func (e *ToolExecutor) executeOne(ctx context.Context, reg *Registry, c ToolCall) (res ToolResult) {
	defer func() {
		if r := recover(); r != nil {
			res = ToolResult{ID: c.ID, Name: c.Name, Err: fmt.Errorf("tool panic recovered: %v", r)}
		}
	}()

	t, ok := reg.Get(c.Name)
	if !ok {
		return ToolResult{ID: c.ID, Name: c.Name, Err: fmt.Errorf("tool not found: %s", c.Name)}
	}
	baseCtx := ctx
	if e.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		baseCtx, cancel = context.WithTimeout(ctx, e.cfg.Timeout)
		defer cancel()
	}
	ro := retry.Options{MaxAttempts: e.cfg.MaxRetries, Backoff: 50 * time.Millisecond}
	var result any
	var err error
	reErr := retry.Do(baseCtx, ro, func() error {
		result, err = t.Execute(baseCtx, c.Input)
		return err
	})
	if reErr != nil {
		err = reErr
	}
	return ToolResult{ID: c.ID, Name: c.Name, Result: result, Err: err}
}

// ExecuteTool 执行单个工具（供 Agent 循环使用）
func (e *ToolExecutor) ExecuteTool(ctx context.Context, reg *Registry, name string, input map[string]any) (any, error) {
	r := e.executeOne(ctx, reg, ToolCall{Name: name, Input: input})
	return r.Result, r.Err
}
