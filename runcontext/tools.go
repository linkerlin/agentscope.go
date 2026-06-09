package runcontext

import (
	"context"

	"github.com/linkerlin/agentscope.go/tool"
)

type toolsKey struct{}

// WithTools attaches per-request session tools (e.g. workspace MCP) to ctx.
func WithTools(ctx context.Context, tools []tool.Tool) context.Context {
	if ctx == nil || len(tools) == 0 {
		return ctx
	}
	return context.WithValue(ctx, toolsKey{}, tools)
}

// Tools returns session-scoped tools from ctx.
func Tools(ctx context.Context) []tool.Tool {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(toolsKey{}).([]tool.Tool)
	return v
}
