package hook

import (
	"context"
	"testing"
)

func TestManagerRegisterAndAll(t *testing.T) {
	m := NewManager()
	m.Register(nil)
	h := HookFunc(func(ctx context.Context, hCtx *HookContext) (*HookResult, error) {
		return nil, nil
	})
	m.Register(h)
	if len(m.All()) != 1 {
		t.Fatal(len(m.All()))
	}
	m.Clear()
	if len(m.All()) != 0 {
		t.Fatal()
	}
}
