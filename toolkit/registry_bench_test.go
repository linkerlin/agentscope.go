package toolkit

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
)

func BenchmarkRegistryToolSpecs(b *testing.B) {
	r := NewRegistry()
	_ = r.Register(tool.NewFunctionTool("f", "d", map[string]any{}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse(""), nil
	}))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.ToolSpecs()
	}
}
