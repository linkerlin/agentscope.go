package toolkit

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
)

func TestRegistryToolSpecs(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(tool.NewFunctionTool("a", "d", map[string]any{}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse(1), nil
	}))
	specs := r.ToolSpecs()
	if len(specs) != 1 || specs[0].Name != "a" {
		t.Fatal(specs)
	}
}

func TestGroupActive(t *testing.T) {
	tk := NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("x", "", map[string]any{}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse(""), nil
	}))
	_ = tk.Groups.CreateGroup("g1", "")
	_ = tk.Groups.AddTool("g1", "x")
	_ = tk.Groups.SetGroupActive("g1", true)
	if len(tk.ActiveTools()) != 1 {
		t.Fatal(len(tk.ActiveTools()))
	}
}

func TestExecutorParallel(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(tool.NewFunctionTool("p", "", map[string]any{}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	}))
	ex := NewToolExecutor(ExecutionConfig{Parallel: true, MaxParallel: 4, MaxRetries: 1})
	res, err := ex.Execute(context.Background(), reg, []ToolCall{{Name: "p"}, {Name: "p"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Err != nil {
		t.Fatal(res)
	}
}


func TestRegistry_MustRegister(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(tool.NewFunctionTool("must", "", nil, nil))
	if _, ok := r.Get("must"); !ok {
		t.Fatal("expected must tool")
	}
}

func TestRegistry_MustRegisterPanic(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(tool.NewFunctionTool("dup", "", nil, nil))
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic")
		}
	}()
	r.MustRegister(tool.NewFunctionTool("dup", "", nil, nil))
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	t1 := tool.NewFunctionTool("dup", "", nil, nil)
	if err := r.Register(t1); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(t1); err == nil {
		t.Fatal("expected error for duplicate")
	}
}
