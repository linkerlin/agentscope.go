package toolkit

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// Toolkit 聚合注册表、分组与执行器
type Toolkit struct {
	Registry *Registry
	Groups   *GroupManager
	Executor *ToolExecutor
}

// NewToolkit 使用默认执行配置创建 Toolkit
func NewToolkit() *Toolkit {
	reg := NewRegistry()
	return &Toolkit{
		Registry: reg,
		Groups:   NewGroupManager(reg),
		Executor: NewToolExecutor(DefaultExecutionConfig()),
	}
}

// NewToolkitWithExecutor 自定义执行器（例如更大超时）
func NewToolkitWithExecutor(exec *ToolExecutor) *Toolkit {
	reg := NewRegistry()
	return &Toolkit{
		Registry: reg,
		Groups:   NewGroupManager(reg),
		Executor: exec,
	}
}

// Register 向注册表注册工具
func (tk *Toolkit) Register(t tool.Tool) error {
	return tk.Registry.Register(t)
}

// ActiveTools 当前应对外暴露的工具实例
func (tk *Toolkit) ActiveTools() []tool.Tool {
	return tk.Groups.ActiveTools()
}

// ActiveToolSpecs 当前应对模型暴露的 ToolSpec 列表
func (tk *Toolkit) ActiveToolSpecs() []model.ToolSpec {
	ts := tk.ActiveTools()
	specs := make([]model.ToolSpec, 0, len(ts))
	for _, t := range ts {
		specs = append(specs, t.Spec())
	}
	return specs
}

// Execute 批量执行（顺序或并行）
func (tk *Toolkit) Execute(ctx context.Context, calls []ToolCall) ([]ToolResult, error) {
	return tk.Executor.Execute(ctx, tk.Registry, calls)
}

// ExecuteTool 执行单个工具名
func (tk *Toolkit) ExecuteTool(ctx context.Context, name string, input map[string]any) (any, error) {
	return tk.Executor.ExecuteTool(ctx, tk.Registry, name, input)
}
