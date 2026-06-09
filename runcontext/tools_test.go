package runcontext

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

type stubTool struct{ name string }

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return s.name }
func (s *stubTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: s.name}
}
func (s *stubTool) Execute(context.Context, map[string]any) (*tool.Response, error) {
	return tool.NewTextResponse("ok"), nil
}

func TestWithTools(t *testing.T) {
	ctx := WithTools(context.Background(), []tool.Tool{&stubTool{name: "x"}})
	tools := Tools(ctx)
	if len(tools) != 1 || tools[0].Name() != "x" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}
