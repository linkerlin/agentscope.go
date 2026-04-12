package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

func TestReMeHookBeforeModel(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, err := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("ping").Build())

	h := NewReMeHook(m).(*ReMeHook)
	ctx := context.Background()
	hist, _ := m.GetAll()
	res, err := h.OnEvent(ctx, &hook.HookContext{
		Point:    hook.HookBeforeModel,
		Messages: hist,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.InjectMessages) == 0 {
		t.Fatal(res)
	}
}

func TestReMeHookSkipsOtherPoints(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	h := NewReMeHook(m).(*ReMeHook)
	res, err := h.OnEvent(context.Background(), &hook.HookContext{
		Point:    hook.HookAfterModel,
		Messages: []*message.Msg{},
	})
	if err != nil || res != nil {
		t.Fatal(res, err)
	}
}

func TestReMeHookNilSafe(t *testing.T) {
	var h *ReMeHook
	res, err := h.OnEvent(context.Background(), &hook.HookContext{Point: hook.HookBeforeModel})
	if err != nil || res != nil {
		t.Fatal(res, err)
	}
	h = &ReMeHook{}
	res, err = h.OnEvent(context.Background(), &hook.HookContext{Point: hook.HookBeforeModel})
	if err != nil || res != nil {
		t.Fatal(res, err)
	}
}
