package toolkit

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
)

func TestNewToolkitWithExecutor(t *testing.T) {
	exec := NewToolExecutor(DefaultExecutionConfig())
	tk := NewToolkitWithExecutor(exec)
	if tk == nil || tk.Executor != exec {
		t.Fatal("expected toolkit with custom executor")
	}
}

func TestToolkit_ActiveToolSpecs(t *testing.T) {
	tk := NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("echo", "echo desc", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	}))
	_ = tk.Groups.CreateGroup("g1", "")
	_ = tk.Groups.AddTool("g1", "echo")
	_ = tk.Groups.SetGroupActive("g1", true)

	specs := tk.ActiveToolSpecs()
	if len(specs) != 1 || specs[0].Name != "echo" {
		t.Fatalf("unexpected specs: %+v", specs)
	}
}

func TestToolkit_Execute(t *testing.T) {
	tk := NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("echo", "", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	}))
	res, err := tk.Execute(context.Background(), []ToolCall{{Name: "echo"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestToolkit_ExecuteTool(t *testing.T) {
	tk := NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("echo", "", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	}))
	resp, err := tk.ExecuteTool(context.Background(), "echo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
}

func TestGroupManager_CreateGroupDuplicate(t *testing.T) {
	tk := NewToolkit()
	if err := tk.Groups.CreateGroup("g1", ""); err != nil {
		t.Fatal(err)
	}
	if err := tk.Groups.CreateGroup("g1", ""); err == nil {
		t.Fatal("expected error for duplicate group")
	}
}

func TestGroupManager_AddTool_Invalid(t *testing.T) {
	tk := NewToolkit()
	if err := tk.Groups.AddTool("missing", "t"); err == nil {
		t.Fatal("expected error for missing group")
	}
	_ = tk.Groups.CreateGroup("g1", "")
	if err := tk.Groups.AddTool("g1", "missing"); err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestGroupManager_SetGroupActive_Invalid(t *testing.T) {
	tk := NewToolkit()
	if err := tk.Groups.SetGroupActive("missing", true); err == nil {
		t.Fatal("expected error for missing group")
	}
}

func TestGroupManager_ActiveTools_NoGroup(t *testing.T) {
	tk := NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("t1", "", nil, nil))
	// no groups created: returns all registry tools
	if len(tk.Groups.ActiveTools()) != 1 {
		t.Fatalf("expected 1 active tool, got %d", len(tk.Groups.ActiveTools()))
	}
}

func TestGroupManager_ActiveTools_ActiveGroup(t *testing.T) {
	tk := NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("t1", "", nil, nil))
	_ = tk.Register(tool.NewFunctionTool("t2", "", nil, nil))
	_ = tk.Groups.CreateGroup("g1", "")
	_ = tk.Groups.AddTool("g1", "t1")
	_ = tk.Groups.SetGroupActive("g1", true)
	active := tk.Groups.ActiveTools()
	if len(active) != 1 {
		t.Fatalf("expected 1 active tool, got %d", len(active))
	}
	if active[0].Name() != "t1" {
		t.Fatalf("unexpected tool: %s", active[0].Name())
	}
}
