package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/service"
)

// ErrStorageNotAvailable is returned when a session-state operation is
// requested but no Storage backend has been configured.
var ErrStorageNotAvailable = errors.New("session_state: storage not available")

// SessionStateManager 管理 Gateway 层 session 与 AgentState 快照的生命周期，
// 支持挂起保存、断线重连恢复、以及 resume 后的清理。
type SessionStateManager struct {
	storage service.Storage
}

// NewSessionStateManager 创建 session 状态管理器。storage 可为 nil（此时所有操作无持久化）。
func NewSessionStateManager(storage service.Storage) *SessionStateManager {
	return &SessionStateManager{storage: storage}
}

// IsAvailable 返回是否配置了持久化存储。
func (m *SessionStateManager) IsAvailable() bool {
	return m != nil && m.storage != nil
}

// SaveSnapshot 将 Agent 当前状态保存为快照。
func (m *SessionStateManager) SaveSnapshot(ctx context.Context, sessionID string, v2 agent.V2Agent) error {
	if m == nil || !m.IsAvailable() {
		return nil
	}
	if sessionID == "" {
		return fmt.Errorf("session_state: empty sessionID")
	}
	st, err := v2.SaveState()
	if err != nil {
		return fmt.Errorf("session_state: SaveState failed: %w", err)
	}
	snap := &service.AgentSnapshot{
		SessionID: sessionID,
		ReplyID:   st.ReplyID,
		State:     st,
		CreatedAt: time.Now(),
	}
	if err := m.storage.SaveSnapshot(ctx, snap); err != nil {
		return fmt.Errorf("session_state: SaveSnapshot failed: %w", err)
	}
	return nil
}

// LoadSnapshot 从存储加载快照并恢复到 Agent。
func (m *SessionStateManager) LoadSnapshot(ctx context.Context, sessionID string, v2 agent.V2Agent) (*service.AgentSnapshot, error) {
	if m == nil || !m.IsAvailable() {
		return nil, ErrStorageNotAvailable
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session_state: empty sessionID")
	}
	snap, err := m.storage.GetSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session_state: GetSnapshot failed: %w", err)
	}
	if snap.State == nil {
		return nil, fmt.Errorf("session_state: snapshot state is nil")
	}
	if err := v2.LoadState(snap.State); err != nil {
		return nil, fmt.Errorf("session_state: LoadState failed: %w", err)
	}
	return snap, nil
}

// HasPendingSnapshot 检查指定 session 是否存在未完成的挂起快照。
func (m *SessionStateManager) HasPendingSnapshot(ctx context.Context, sessionID string) bool {
	if m == nil || !m.IsAvailable() || sessionID == "" {
		return false
	}
	snap, err := m.storage.GetSnapshot(ctx, sessionID)
	return err == nil && snap != nil && snap.State != nil && snap.State.SuspendedAt != nil
}

// Resume 加载快照并向 Agent 注入恢复事件，成功后清理快照。
// 行为：
//   - 若 storage 可用且存在 pending snapshot（ reconnect 场景），先 LoadState 再 InjectEvent，然后清理快照。
//   - 若 storage 不可用或没有 pending snapshot（同连接内 resume），直接 InjectEvent。
func (m *SessionStateManager) Resume(ctx context.Context, sessionID string, v2 agent.V2Agent, ev event.AgentEvent) error {
	if m != nil && m.IsAvailable() && sessionID != "" && m.HasPendingSnapshot(ctx, sessionID) {
		if _, err := m.LoadSnapshot(ctx, sessionID, v2); err != nil {
			return err
		}
		if err := v2.InjectEvent(ctx, ev); err != nil {
			return fmt.Errorf("session_state: InjectEvent failed: %w", err)
		}
		// 注入成功后立即清理快照，避免重复恢复
		_ = m.DeleteSnapshot(ctx, sessionID)
		return nil
	}
	// 无存 storage 或无 pending snapshot：依赖同连接内存 resume
	return v2.InjectEvent(ctx, ev)
}

// DeleteSnapshot 删除指定 session 的快照（幂等）。
func (m *SessionStateManager) DeleteSnapshot(ctx context.Context, sessionID string) error {
	if m == nil || !m.IsAvailable() || sessionID == "" {
		return nil
	}
	return m.storage.DeleteSnapshot(ctx, sessionID)
}
