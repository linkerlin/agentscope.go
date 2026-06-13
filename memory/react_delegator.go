package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/message"
	"golang.org/x/sync/errgroup"
)

// ReactDelegator 任务分派器
// 根据 memory_target / memory_type 路由到对应 Summarizer
type ReactDelegator struct {
	// 处理器注册表
	handlers map[MemoryType]ReactTaskHandler
	mu       sync.RWMutex
}

// ReactTaskHandler 任务处理器函数
type ReactTaskHandler func(ctx context.Context, task *ReactTask) (*ReactTaskResult, error)

// ReactTask 分派任务
type ReactTask struct {
	ID          string      `json:"id"`
	Type        MemoryType  `json:"type"`
	Target      string      `json:"target"`
	Content     string      `json:"content"`
	Messages    []*message.Msg  `json:"messages,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ReactTaskResult 任务结果
type ReactTaskResult struct {
	TaskID      string        `json:"task_id"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	MemoryNodes []*MemoryNode `json:"memory_nodes,omitempty"`
	Profile     map[string]any `json:"profile,omitempty"`
}

// NewReactDelegator 创建任务分派器
func NewReactDelegator() *ReactDelegator {
	return &ReactDelegator{
		handlers: make(map[MemoryType]ReactTaskHandler),
	}
}

// Register 注册任务处理器
func (d *ReactDelegator) Register(taskType MemoryType, handler ReactTaskHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[taskType] = handler
}

// RegisterDefaultHandlers 注册默认处理器
func (d *ReactDelegator) RegisterDefaultHandlers(
	personalSum *PersonalSummarizer,
	proceduralSum *ProceduralSummarizer,
	toolSum *ToolSummarizer,
) {
	// Personal 处理器
	if personalSum != nil {
		d.Register(MemoryTypePersonal, func(ctx context.Context, task *ReactTask) (*ReactTaskResult, error) {
			// 这里简化实现，实际应调用 personalSum.Summarize
			node := NewMemoryNode(MemoryTypePersonal, task.Target, task.Content)
			return &ReactTaskResult{
				TaskID:      task.ID,
				Success:     true,
				MemoryNodes: []*MemoryNode{node},
			}, nil
		})
	}

	// Procedural 处理器
	if proceduralSum != nil {
		d.Register(MemoryTypeProcedural, func(ctx context.Context, task *ReactTask) (*ReactTaskResult, error) {
			node := NewMemoryNode(MemoryTypeProcedural, task.Target, task.Content)
			return &ReactTaskResult{
				TaskID:      task.ID,
				Success:     true,
				MemoryNodes: []*MemoryNode{node},
			}, nil
		})
	}

	// Tool 处理器
	if toolSum != nil {
		d.Register(MemoryTypeTool, func(ctx context.Context, task *ReactTask) (*ReactTaskResult, error) {
			node := NewMemoryNode(MemoryTypeTool, task.Target, task.Content)
			return &ReactTaskResult{
				TaskID:      task.ID,
				Success:     true,
				MemoryNodes: []*MemoryNode{node},
			}, nil
		})
	}
}

// DelegateTask 分派任务到对应处理器
func (d *ReactDelegator) DelegateTask(ctx context.Context, task *ReactTask) (*ReactTaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("nil task")
	}

	d.mu.RLock()
	handler, ok := d.handlers[task.Type]
	d.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler for task type: %s", task.Type)
	}

	return handler(ctx, task)
}

// DelegateBatch 批量分派任务（并行执行）
func (d *ReactDelegator) DelegateBatch(ctx context.Context, tasks []*ReactTask) ([]*ReactTaskResult, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	results := make([]*ReactTaskResult, len(tasks))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	for i, task := range tasks {
		i, task := i, task // 捕获循环变量
		g.Go(func() error {
			result, err := d.DelegateTask(ctx, task)
			mu.Lock()
			if err != nil {
				results[i] = &ReactTaskResult{
					TaskID:  task.ID,
					Success: false,
					Error:   err.Error(),
				}
			} else {
				results[i] = result
			}
			mu.Unlock()
			return nil // 不因单个失败取消其他
		})
	}

	_ = g.Wait()
	return results, nil
}

// GetRegisteredTypes 返回已注册的任务类型
func (d *ReactDelegator) GetRegisteredTypes() []MemoryType {
	d.mu.RLock()
	defer d.mu.RUnlock()

	types := make([]MemoryType, 0, len(d.handlers))
	for t := range d.handlers {
		types = append(types, t)
	}
	return types
}

// HasHandler 检查是否有指定类型的处理器
func (d *ReactDelegator) HasHandler(taskType MemoryType) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.handlers[taskType]
	return ok
}

// Stats 返回分派器统计
func (d *ReactDelegator) Stats() map[string]any {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]any{
		"registered_types": len(d.handlers),
		"types":            d.GetRegisteredTypes(),
	}
}
