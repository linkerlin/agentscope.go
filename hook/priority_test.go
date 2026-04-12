package hook

import (
	"context"
	"testing"
)

func TestSortByPriority(t *testing.T) {
	var order []int
	a := HookFunc(func(ctx context.Context, hCtx *HookContext) (*HookResult, error) {
		order = append(order, 1)
		return nil, nil
	})
	b := WithPriority(HookFunc(func(ctx context.Context, hCtx *HookContext) (*HookResult, error) {
		order = append(order, 2)
		return nil, nil
	}), 50)
	c := WithPriority(HookFunc(func(ctx context.Context, hCtx *HookContext) (*HookResult, error) {
		order = append(order, 3)
		return nil, nil
	}), 200)

	sorted := SortByPriority([]Hook{b, a, c})
	for _, h := range sorted {
		_, _ = h.OnEvent(context.Background(), &HookContext{})
	}
	if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 3 {
		t.Fatalf("order %v", order)
	}
}
