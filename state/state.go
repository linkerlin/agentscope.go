package state

// State 标记可持久化的状态类型（用于类型标识与序列化约定）
type State interface {
	StateType() string
}
