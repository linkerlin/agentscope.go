package state

// Store 键值状态存储（演进方案中的 Session 持久化能力；为避免与对话 session 包冲突命名为 Store）
type Store interface {
	Save(key string, value State) error
	Get(key string, dest State) error
	Exists(key string) bool
	Delete(key string) error
	ListKeys() []string
}
