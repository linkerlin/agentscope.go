package state

// StateModule 可持久化组件（对齐 Java StateModule 语义）
type StateModule interface {
	SaveTo(store Store, key string) error
	LoadFrom(store Store, key string) error
	// LoadIfExists 若存在则加载并返回 true，否则返回 false
	LoadIfExists(store Store, key string) (bool, error)
}
