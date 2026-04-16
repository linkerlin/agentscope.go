package agent

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

type subagentDepthKey struct{}

// WithSubagentDepth 在 ctx 中记录子 Agent 调用深度（内部使用）
func WithSubagentDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, subagentDepthKey{}, depth)
}

// SubagentDepth 读取当前深度（未设置则为 0）
func SubagentDepth(ctx context.Context) int {
	v, ok := ctx.Value(subagentDepthKey{}).(int)
	if !ok {
		return 0
	}
	return v
}

// SubagentTool 将 Agent 暴露为 tool.Tool；input 需包含字符串字段 "query"
type SubagentTool struct {
	name        string
	description string
	inner       Agent
	maxDepth    int
	parameters  map[string]any
}

// NewSubagentTool 创建子 Agent 工具；maxDepth<=0 时默认为 3
func NewSubagentTool(inner Agent, name, description string, maxDepth int) *SubagentTool {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "用户任务或问题"},
		},
		"required": []string{"query"},
	}
	return &SubagentTool{
		name:        name,
		description: description,
		inner:       inner,
		maxDepth:    maxDepth,
		parameters:  params,
	}
}

func (s *SubagentTool) Name() string        { return s.name }
func (s *SubagentTool) Description() string { return s.description }

func (s *SubagentTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: s.name, Description: s.description, Parameters: s.parameters}
}

func (s *SubagentTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	d := SubagentDepth(ctx)
	if d >= s.maxDepth {
		return nil, fmt.Errorf("subagent: max depth %d exceeded", s.maxDepth)
	}
	q, _ := input["query"].(string)
	if q == "" {
		return nil, fmt.Errorf("subagent: missing query")
	}
	ctx = WithSubagentDepth(ctx, d+1)
	msg := message.NewMsg().Role(message.RoleUser).TextContent(q).Build()
	out, err := s.inner.Call(ctx, msg)
	if err != nil {
		return nil, err
	}
	return tool.NewTextResponse(out.GetTextContent()), nil
}

var _ tool.Tool = (*SubagentTool)(nil)
