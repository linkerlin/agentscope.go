package hook

import "sync"

// Manager 管理 Hook 注册与按优先级执行顺序（线程安全）
type Manager struct {
	mu    sync.RWMutex
	hooks []Hook
}

// NewManager 创建空的 Hook 管理器
func NewManager() *Manager {
	return &Manager{hooks: make([]Hook, 0)}
}

// Register 注册 Hook（按优先级重排）
func (m *Manager) Register(hooks ...Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range hooks {
		if h == nil {
			continue
		}
		m.hooks = append(m.hooks, h)
	}
	m.hooks = SortByPriority(m.hooks)
}

// Clear 清空全部 Hook
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = nil
}

// All 返回当前已排序的 Hook 切片副本
func (m *Manager) All() []Hook {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Hook, len(m.hooks))
	copy(out, m.hooks)
	return out
}
