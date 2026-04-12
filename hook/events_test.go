package hook

import (
	"context"
	"testing"
)

func TestSortStreamHooks(t *testing.T) {
	a := WithStreamPriority(StreamHookFunc(func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		return nil, nil
	}), 200)
	b := WithStreamPriority(StreamHookFunc(func(ctx context.Context, ev Event) (*StreamHookResult, error) {
		return nil, nil
	}), 50)
	sorted := SortStreamHooks([]StreamHook{a, b})
	if StreamPriorityOf(sorted[0]) != 50 {
		t.Fatal("order")
	}
}
