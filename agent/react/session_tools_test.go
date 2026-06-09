package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/runcontext"
	"github.com/linkerlin/agentscope.go/tool"
)

type sessionStubTool struct{ name string }

func (s *sessionStubTool) Name() string        { return s.name }
func (s *sessionStubTool) Description() string { return s.name }
func (s *sessionStubTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: s.name, Description: s.name}
}
func (s *sessionStubTool) Execute(_ context.Context, _ map[string]any) (*tool.Response, error) {
	return tool.NewTextResponse("session-ok"), nil
}

func TestReActAgent_SessionToolsFromContext(t *testing.T) {
	a, err := Builder().
		Name("t").
		Model(&mockChatModel{name: "mock"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := runcontext.WithTools(context.Background(), []tool.Tool{
		&sessionStubTool{name: "mcp__demo__ping"},
	})
	specs := a.toolSpecs(ctx)
	if len(specs) != 1 || specs[0].Name != "mcp__demo__ping" {
		t.Fatalf("unexpected specs: %#v", specs)
	}

	resp, err := a.executeTool(ctx, "mcp__demo__ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "session-ok" {
		t.Fatalf("unexpected resp: %s", resp.GetTextContent())
	}
}
