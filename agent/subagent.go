package agent

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
)

type subagentDepthKey struct{}

// WithSubagentDepth 在 ctx 中记录子 Agent 调用深度（内部使用）
func WithSubagentDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, subagentDepthKey{}, depth)
}

// SubagentDepth 读取当前深度（未设置则为 0）
func SubagentDepth(ctx context.Context) int {
	v, ok := ctx.Value(subagentDepthKey{}).(int)
	if !ok {
		return 0
	}
	return v
}

// AgentProvider creates a fresh Agent instance for a new session.
type AgentProvider func() Agent

// SubagentTool 将 Agent 暴露为 tool.Tool；支持 query 或 session_id+message 两种输入模式
type SubagentTool struct {
	name        string
	description string
	inner       Agent
	provider    AgentProvider
	maxDepth    int
	parameters  map[string]any
	sessions    map[string][]*message.Msg
	mu          sync.Mutex
}

// NewSubagentTool 创建子 Agent 工具；maxDepth<=0 时默认为 3
func NewSubagentTool(inner Agent, name, description string, maxDepth int) *SubagentTool {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "用户任务或问题（旧模式，与 session_id/message 互斥）",
			},
			"session_id": map[string]any{
				"type":        "string",
				"description": "会话 ID，用于多轮对话。省略则开启新会话。",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "要发送给子 Agent 的消息",
			},
		},
	}
	return &SubagentTool{
		name:        name,
		description: description,
		inner:       inner,
		maxDepth:    maxDepth,
		parameters:  params,
		sessions:    make(map[string][]*message.Msg),
	}
}

// NewSubagentToolWithProvider 使用 Provider 创建子 Agent 工具，每个新会话获得独立实例
func NewSubagentToolWithProvider(provider AgentProvider, name, description string, maxDepth int) *SubagentTool {
	t := NewSubagentTool(nil, name, description, maxDepth)
	t.provider = provider
	return t
}

func (s *SubagentTool) Name() string        { return s.name }
func (s *SubagentTool) Description() string { return s.description }

func (s *SubagentTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: s.name, Description: s.description, Parameters: s.parameters}
}

func (s *SubagentTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	d := SubagentDepth(ctx)
	if d >= s.maxDepth {
		return nil, fmt.Errorf("subagent: max depth %d exceeded", s.maxDepth)
	}
	ctx = WithSubagentDepth(ctx, d+1)

	// Legacy mode: query
	if q, ok := input["query"].(string); ok && q != "" {
		msg := message.NewMsg().Role(message.RoleUser).TextContent(q).Build()
		out, err := s.getAgent().Call(ctx, msg)
		if err != nil {
			return nil, err
		}
		return tool.NewTextResponse(out.GetTextContent()), nil
	}

	// Session mode
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	m, _ := input["message"].(string)
	if m == "" {
		// try number type from JSON
		if f, ok := input["message"].(float64); ok {
			m = strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	if m == "" {
		return nil, fmt.Errorf("subagent: missing message")
	}

	agent := s.getAgent()

	s.mu.Lock()
	history := s.sessions[sessionID]
	msg := message.NewMsg().Role(message.RoleUser).TextContent(m).Build()
	history = append(history, msg)
	s.sessions[sessionID] = history
	s.mu.Unlock()

	out, err := agent.Call(ctx, msg)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.sessions[sessionID] = append(s.sessions[sessionID], out)
	s.mu.Unlock()

	text := out.GetTextContent()
	result := fmt.Sprintf("session_id: %s\n\n%s", sessionID, text)
	return tool.NewTextResponse(result), nil
}

func (s *SubagentTool) getAgent() Agent {
	if s.provider != nil {
		return s.provider()
	}
	return s.inner
}

var _ tool.Tool = (*SubagentTool)(nil)

// SessionState 用于持久化 SubagentSessionTool 的会话历史
type SessionState struct {
	Sessions map[string][]*message.Msg `json:"sessions"`
}

// StateType 返回状态类型标识
func (s *SessionState) StateType() string { return "subagent_session" }

// SubagentSessionTool 在 SubagentTool 基础上支持通过 state.Store 持久化会话历史
type SubagentSessionTool struct {
	*SubagentTool
	store state.Store
}

// NewSubagentSessionTool 创建支持会话持久化的子 Agent 工具
func NewSubagentSessionTool(inner Agent, name, description string, maxDepth int, store state.Store) *SubagentSessionTool {
	return &SubagentSessionTool{
		SubagentTool: NewSubagentTool(inner, name, description, maxDepth),
		store:        store,
	}
}

// Execute 执行子 Agent 调用，并在有 session_id 时自动加载/保存会话状态
func (s *SubagentSessionTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID != "" && s.store != nil {
		if _, err := s.LoadIfExists(s.store, sessionID); err != nil {
			return nil, err
		}
	}
	resp, err := s.SubagentTool.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	if sessionID != "" && s.store != nil {
		if err := s.SaveTo(s.store, sessionID); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// SaveTo 将当前会话历史保存到 store
func (s *SubagentSessionTool) SaveTo(store state.Store, key string) error {
	if store == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &SessionState{Sessions: make(map[string][]*message.Msg, len(s.sessions))}
	for k, v := range s.sessions {
		st.Sessions[k] = append([]*message.Msg(nil), v...)
	}
	return store.Save(key, st)
}

// LoadFrom 从 store 恢复会话历史
func (s *SubagentSessionTool) LoadFrom(store state.Store, key string) error {
	if store == nil {
		return nil
	}
	var st SessionState
	if err := store.Get(key, &st); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = st.Sessions
	return nil
}

// LoadIfExists 若 key 存在则加载并返回 true
func (s *SubagentSessionTool) LoadIfExists(store state.Store, key string) (bool, error) {
	if store == nil || !store.Exists(key) {
		return false, nil
	}
	return true, s.LoadFrom(store, key)
}

var _ state.StateModule = (*SubagentSessionTool)(nil)
var _ tool.Tool = (*SubagentSessionTool)(nil)
