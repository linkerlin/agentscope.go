package hook

import "context"

// WithStreamPriority 为 StreamHook 绑定优先级（与 WithPriority 对称）
func WithStreamPriority(h StreamHook, priority int) StreamHook {
	if h == nil {
		return nil
	}
	return &streamPriorityHook{inner: h, p: priority}
}

type streamPriorityHook struct {
	inner StreamHook
	p     int
}

func (s *streamPriorityHook) OnStreamEvent(ctx context.Context, ev Event) (*StreamHookResult, error) {
	return s.inner.OnStreamEvent(ctx, ev)
}

func (s *streamPriorityHook) Priority() int { return s.p }
