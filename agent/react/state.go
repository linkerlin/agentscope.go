package react

import (
	"errors"

	"github.com/linkerlin/agentscope.go/state"
)

var errNilStore = errors.New("react agent: nil state store")

var _ state.StateModule = (*ReActAgent)(nil)

// AgentState 可序列化的 Agent 配置快照（不含 ChatModel、Tool 实例，需调用方在 Load 后重新注入）
type AgentState struct {
	AgentID       string         `json:"agent_id"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	SystemPrompt  string         `json:"system_prompt"`
	MaxIterations int            `json:"max_iterations"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// StateType 实现 state.State
func (s AgentState) StateType() string { return "agent_state" }

// SaveTo 将当前 Agent 配置写入 Store
func (a *ReActAgent) SaveTo(store state.Store, key string) error {
	if store == nil {
		return errNilStore
	}
	id := a.agentID
	if id == "" {
		id = a.name
	}
	st := AgentState{
		AgentID:       id,
		Name:          a.name,
		Description:   a.description,
		SystemPrompt:  a.sysPrompt,
		MaxIterations: a.maxIterations,
		Metadata:      a.metadata(),
	}
	return store.Save(key, st)
}

// LoadFrom 从 Store 恢复可序列化字段（不会恢复 model/tools/memory）
func (a *ReActAgent) LoadFrom(store state.Store, key string) error {
	if store == nil {
		return errNilStore
	}
	var st AgentState
	if err := store.Get(key, &st); err != nil {
		return err
	}
	a.applyAgentState(st)
	return nil
}

// LoadIfExists 若存在键则加载并返回 true
func (a *ReActAgent) LoadIfExists(store state.Store, key string) (bool, error) {
	if store == nil {
		return false, errNilStore
	}
	if !store.Exists(key) {
		return false, nil
	}
	if err := a.LoadFrom(store, key); err != nil {
		return false, err
	}
	return true, nil
}

func (a *ReActAgent) applyAgentState(st AgentState) {
	a.agentID = st.AgentID
	a.name = st.Name
	a.description = st.Description
	a.sysPrompt = st.SystemPrompt
	if st.MaxIterations > 0 {
		a.maxIterations = st.MaxIterations
	}
	if len(st.Metadata) > 0 {
		a.meta = cloneAnyMap(st.Metadata)
	}
}

func (a *ReActAgent) metadata() map[string]any {
	if len(a.meta) == 0 {
		return nil
	}
	return cloneAnyMap(a.meta)
}

func cloneAnyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
