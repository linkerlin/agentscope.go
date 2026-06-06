package toolkit

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// Toolkit 聚合注册表、分组与执行器
type Toolkit struct {
	Registry    *Registry
	Groups      *GroupManager
	Executor    *ToolExecutor
	middlewares []Middleware
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

// Use registers one or more middleware. Middleware are applied in registration order.
func (tk *Toolkit) Use(mw ...Middleware) {
	tk.middlewares = append(tk.middlewares, mw...)
}

// Execute 批量执行（顺序或并行），经过中间件链。
func (tk *Toolkit) Execute(ctx context.Context, calls []ToolCall) ([]ToolResult, error) {
	if len(tk.middlewares) == 0 {
		return tk.Executor.Execute(ctx, tk.Registry, calls)
	}

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		results, err := tk.Executor.Execute(ctx, tk.Registry, req.ToolCalls)
		return &Response{Results: results}, err
	}
	handler = chain(handler, tk.middlewares...)

	resp, err := handler(ctx, &Request{Stage: StageExecute, ToolCalls: calls})
	if err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// ExecuteTool 执行单个工具名，经过中间件链。
func (tk *Toolkit) ExecuteTool(ctx context.Context, name string, input map[string]any) (*tool.Response, error) {
	if len(tk.middlewares) == 0 {
		return tk.Executor.ExecuteTool(ctx, tk.Registry, name, input)
	}

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		resp, err := tk.Executor.ExecuteTool(ctx, tk.Registry, req.ToolName, req.ToolInput)
		return &Response{Single: resp}, err
	}
	handler = chain(handler, tk.middlewares...)

	resp, err := handler(ctx, &Request{Stage: StageExecuteTool, ToolName: name, ToolInput: input})
	if err != nil {
		return nil, err
	}
	return resp.Single, nil
}
