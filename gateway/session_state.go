package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/service"
)

// ErrStorageNotAvailable is returned when a session-state operation is
// requested but no Storage backend has been configured. Defined in
// gateway/sessionapi (whose resume handler maps it to 503); aliased here so
// both packages compare against the same error value.
var ErrStorageNotAvailable = sessionapi.ErrStorageNotAvailable

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

// Resume delivers a HITL resume command for the session (23.2). With storage
// configured it is an idempotent state machine over the persisted command:
//
//   - first confirm for a confirm_id: persist the command (pending), deliver
//     it to the live waiter, mark it executing;
//   - a repeat confirm while the command is executing (retry after a crash
//     mid-execution, or a double click): refused without re-delivery — the
//     resumed tool runs at most once;
//   - after the resumed turn completes, callers remove the snapshot
//     (DeleteSnapshot); later confirms fall back to the in-memory path,
//     where a consumed waiter is gone and injection fails.
//
// Without storage the old in-memory path applies (single-connection resume).
func (m *SessionStateManager) Resume(ctx context.Context, sessionID string, v2 agent.V2Agent, ev event.AgentEvent) error {
	if m == nil || !m.IsAvailable() || sessionID == "" {
		// No persistence: in-memory resume exactly as before (22.x
		// nil-receiver semantics preserved).
		return v2.InjectEvent(ctx, ev)
	}

	confirm := resumeConfirmID(ev)
	if confirm == "" {
		return fmt.Errorf("session_state: resume event carries no confirm id")
	}

	snap, err := m.storage.GetSnapshot(ctx, sessionID)
	if err == nil && snap != nil && snap.PendingResume != nil &&
		snap.PendingResume.ConfirmID == confirm && snap.PendingResume.State == service.ResumeExecuting {
		// Idempotency hit: this command was already delivered and is
		// mid-execution. Never re-deliver (at-most-once).
		return sessionapi.ErrResumeAlreadyExecuting
	}

	if err == nil && snap != nil && snap.State != nil && snap.State.SuspendedAt != nil {
		// Load the suspended state into this agent, then persist the command
		// BEFORE delivery: a crash between persist and delivery leaves a
		// retryable pending command; a crash after delivery leaves an
		// executing one that blocks double execution.
		if _, err := m.LoadSnapshot(ctx, sessionID, v2); err != nil {
			return err
		}
		cmd := snap.PendingResume
		if cmd == nil || cmd.ConfirmID != confirm {
			cmd = &service.ResumeCommand{
				ConfirmID: confirm,
				Version:   nextResumeVersion(snap),
				State:     service.ResumePending,
				CreatedAt: time.Now().UTC(),
			}
		}
		cmd.State = service.ResumeExecuting
		now := time.Now().UTC()
		cmd.ExecutedAt = &now
		snap.PendingResume = cmd
		if err := m.storage.SaveSnapshot(ctx, snap); err != nil {
			return fmt.Errorf("session_state: persist resume command: %w", err)
		}
	}

	// Deliver to the live waiter. No waiter on this replica (the run parked
	// elsewhere, already consumed the confirmation, or died mid-execution):
	// the persisted command stays on the snapshot for the replica that
	// actually holds the run, and the caller gets ErrResumeNotDelivered.
	if err := v2.InjectEvent(ctx, ev); err != nil {
		if errors.Is(err, agent.ErrNoWaiter) {
			return fmt.Errorf("%w (%s): %w", sessionapi.ErrResumeNotDelivered, sessionID, err)
		}
		return fmt.Errorf("session_state: InjectEvent failed: %w", err)
	}
	return nil
}

// nextResumeVersion returns the snapshot's resume-command version + 1.
func nextResumeVersion(snap *service.AgentSnapshot) int64 {
	if snap != nil && snap.PendingResume != nil {
		return snap.PendingResume.Version + 1
	}
	return 1
}

// resumeConfirmID extracts the idempotency key from a resume event.
func resumeConfirmID(ev event.AgentEvent) string {
	switch e := ev.(type) {
	case *event.UserConfirmResultEvent:
		return e.ConfirmID
	case *event.ExternalExecutionResultEvent:
		return e.ConfirmID
	default:
		return ""
	}
}

// DeleteSnapshot 删除指定 session 的快照（幂等）。Resumed turns call this on
// COMPLETION (23.2): deleting after the work finished — not after injection —
// keeps a crash window auditable and later duplicates inert.
func (m *SessionStateManager) DeleteSnapshot(ctx context.Context, sessionID string) error {
	if m == nil || !m.IsAvailable() || sessionID == "" {
		return nil
	}
	return m.storage.DeleteSnapshot(ctx, sessionID)
}
