package hook

import "sort"

// DefaultPriority 未实现 Prioritized 的 Hook 使用的默认优先级（数值越小越先执行）
const DefaultPriority = 100

// Prioritized 可选实现：为 Hook 提供优先级
type Prioritized interface {
	Priority() int
}

// WithPriority 将任意 Hook 包装为带优先级的 Hook（便于链式注册）
func WithPriority(h Hook, priority int) Hook {
	if h == nil {
		return nil
	}
	return &priorityHook{Hook: h, p: priority}
}

type priorityHook struct {
	Hook
	p int
}

func (p *priorityHook) Priority() int { return p.p }

// PriorityOf 返回 Hook 的优先级；未实现 Prioritized 时返回 DefaultPriority
func PriorityOf(h Hook) int {
	if h == nil {
		return DefaultPriority
	}
	if ph, ok := h.(Prioritized); ok {
		return ph.Priority()
	}
	return DefaultPriority
}

// SortByPriority 返回按优先级排序的 Hook 切片副本（稳定排序：同优先级保持原顺序）
func SortByPriority(hooks []Hook) []Hook {
	if len(hooks) <= 1 {
		out := make([]Hook, len(hooks))
		copy(out, hooks)
		return out
	}
	type item struct {
		h Hook
		i int
	}
	items := make([]item, len(hooks))
	for i, h := range hooks {
		items[i] = item{h: h, i: i}
	}
	sort.SliceStable(items, func(i, j int) bool {
		pi, pj := PriorityOf(items[i].h), PriorityOf(items[j].h)
		if pi != pj {
			return pi < pj
		}
		return items[i].i < items[j].i
	})
	out := make([]Hook, len(items))
	for i := range items {
		out[i] = items[i].h
	}
	return out
}
