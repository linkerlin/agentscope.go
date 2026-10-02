package toolkit

import (
	"context"
	"fmt"

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

// ExecuteToolChunked runs one tool with live output (20.6): when the tool
// implements tool.ChunkedTool its chunks flow to emit as they are produced
// (synchronously from the executing goroutine — order is emit order), and
// the final Response is returned as usual. Plain tools execute exactly like
// ExecuteTool. Toolkit middlewares wrap the execution the same way.
func (tk *Toolkit) ExecuteToolChunked(ctx context.Context, name string, input map[string]any, emit func(chunk string)) (*tool.Response, error) {
	resolve := func() (tool.Tool, bool) {
		t, ok := tk.Registry.Get(name)
		return t, ok
	}
	if len(tk.middlewares) == 0 {
		return executeChunked(ctx, resolve, input, emit)
	}
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		r := func() (tool.Tool, bool) { return tk.Registry.Get(req.ToolName) }
		resp, err := executeChunked(ctx, r, req.ToolInput, emit)
		return &Response{Single: resp}, err
	}
	handler = chain(handler, tk.middlewares...)
	resp, err := handler(ctx, &Request{Stage: StageExecuteTool, ToolName: name, ToolInput: input})
	if err != nil {
		return nil, err
	}
	return resp.Single, nil
}

// executeChunked dispatches to the chunked execution when the tool supports
// it, otherwise to the plain Execute.
func executeChunked(ctx context.Context, resolve func() (tool.Tool, bool), input map[string]any, emit func(chunk string)) (*tool.Response, error) {
	t, ok := resolve()
	if !ok {
		return nil, fmt.Errorf("tool not found")
	}
	if ct, ok := t.(tool.ChunkedTool); ok {
		return ct.ExecuteChunked(ctx, input, emit)
	}
	return t.Execute(ctx, input)
}
