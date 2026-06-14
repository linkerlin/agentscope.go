package gateway

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

// mockStorage implements just the snapshot methods of service.Storage.
type mockStorage struct {
	saved   map[string]*service.AgentSnapshot
	deleted []string
}

func newMockStorage() *mockStorage {
	return &mockStorage{saved: make(map[string]*service.AgentSnapshot)}
}

func (m *mockStorage) SaveSnapshot(ctx context.Context, snap *service.AgentSnapshot) error {
	m.saved[snap.SessionID] = snap
	return nil
}

func (m *mockStorage) GetSnapshot(ctx context.Context, sessionID string) (*service.AgentSnapshot, error) {
	snap, ok := m.saved[sessionID]
	if !ok {
		return nil, context.Canceled // arbitrary sentinel
	}
	return snap, nil
}

func (m *mockStorage) DeleteSnapshot(ctx context.Context, sessionID string) error {
	delete(m.saved, sessionID)
	m.deleted = append(m.deleted, sessionID)
	return nil
}

// Stub implementations for the rest of service.Storage
func (m *mockStorage) SaveUser(ctx context.Context, user *service.User) error          { return nil }
func (m *mockStorage) GetUser(ctx context.Context, id string) (*service.User, error)   { return nil, nil }
func (m *mockStorage) ListUsers(ctx context.Context) ([]*service.User, error)          { return nil, nil }
func (m *mockStorage) DeleteUser(ctx context.Context, id string) error                 { return nil }
func (m *mockStorage) SaveSession(ctx context.Context, session *service.Session) error { return nil }
func (m *mockStorage) GetSession(ctx context.Context, id string) (*service.Session, error) {
	return nil, nil
}
func (m *mockStorage) ListSessionsByUser(ctx context.Context, userID string) ([]*service.Session, error) {
	return nil, nil
}
func (m *mockStorage) DeleteSession(ctx context.Context, id string) error { return nil }
func (m *mockStorage) SaveAgentConfig(ctx context.Context, cfg *service.AgentConfig) error {
	return nil
}
func (m *mockStorage) GetAgentConfig(ctx context.Context, id string) (*service.AgentConfig, error) {
	return nil, nil
}
func (m *mockStorage) ListAgentConfigsByUser(ctx context.Context, userID string) ([]*service.AgentConfig, error) {
	return nil, nil
}
func (m *mockStorage) DeleteAgentConfig(ctx context.Context, id string) error             { return nil }
func (m *mockStorage) SaveCredential(ctx context.Context, cred *service.Credential) error { return nil }
func (m *mockStorage) GetCredential(ctx context.Context, id string) (*service.Credential, error) {
	return nil, nil
}
func (m *mockStorage) ListCredentialsByUser(ctx context.Context, userID string) ([]*service.Credential, error) {
	return nil, nil
}
func (m *mockStorage) DeleteCredential(ctx context.Context, id string) error             { return nil }
func (m *mockStorage) SaveMessage(ctx context.Context, msg *service.StoredMessage) error { return nil }
func (m *mockStorage) GetMessage(ctx context.Context, id string) (*service.StoredMessage, error) {
	return nil, nil
}
func (m *mockStorage) UpsertMessage(ctx context.Context, msg *service.StoredMessage) error {
	return nil
}
func (m *mockStorage) ListMessagesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*service.StoredMessage, error) {
	return nil, nil
}
func (m *mockStorage) DeleteMessagesBySession(ctx context.Context, sessionID string) error {
	return nil
}
func (m *mockStorage) SaveSchedule(ctx context.Context, sched *service.Schedule) error { return nil }
func (m *mockStorage) GetSchedule(ctx context.Context, id string) (*service.Schedule, error) {
	return nil, fmt.Errorf("schedule not found: %s", id)
}
func (m *mockStorage) ListSchedulesByUser(ctx context.Context, userID string) ([]*service.Schedule, error) {
	return nil, nil
}
func (m *mockStorage) ListAllSchedules(ctx context.Context) ([]*service.Schedule, error) {
	return nil, nil
}
func (m *mockStorage) DeleteSchedule(ctx context.Context, id string) error { return nil }
func (m *mockStorage) GetUserByEmail(ctx context.Context, email string) (*service.User, error) {
	return nil, nil
}
func (m *mockStorage) ListSessionsBySchedule(ctx context.Context, userID, scheduleID string) ([]*service.Session, error) {
	return nil, nil
}

// stateTestV2Agent is a minimal V2Agent stub used only in this test file.
type stateTestV2Agent struct {
	state     *agent.AgentState
	injected  []event.AgentEvent
	injectErr error
	loadErr   error
}

func (a *stateTestV2Agent) Name() string { return "state-test" }
func (a *stateTestV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (a *stateTestV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}
func (a *stateTestV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (a *stateTestV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	return nil, nil
}
func (a *stateTestV2Agent) SaveState() (*agent.AgentState, error) {
	return a.state, nil
}
func (a *stateTestV2Agent) LoadState(st *agent.AgentState) error {
	if a.loadErr != nil {
		return a.loadErr
	}
	a.state = st
	return nil
}
func (a *stateTestV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	if a.injectErr != nil {
		return a.injectErr
	}
	a.injected = append(a.injected, ev)
	return nil
}

func TestSessionStateManager_NilReceiver(t *testing.T) {
	var m *SessionStateManager
	if m.IsAvailable() {
		t.Fatal("expected nil manager not available")
	}
	if err := m.SaveSnapshot(context.Background(), "s", nil); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteSnapshot(context.Background(), "s"); err != nil {
		t.Fatal(err)
	}
	if m.HasPendingSnapshot(context.Background(), "s") {
		t.Fatal("expected no pending snapshot")
	}
}

func TestSessionStateManager_NoStorage(t *testing.T) {
	m := NewSessionStateManager(nil)
	if m.IsAvailable() {
		t.Fatal("expected not available")
	}
	if err := m.SaveSnapshot(context.Background(), "s", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LoadSnapshot(context.Background(), "s", nil); err != ErrStorageNotAvailable {
		t.Fatalf("expected ErrStorageNotAvailable, got %v", err)
	}
}

func TestSessionStateManager_SaveAndLoad(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	v2 := &stateTestV2Agent{state: &agent.AgentState{
		ReplyID:       "r1",
		WaitConfirmID: "c1",
		SuspendedAt:   &[]time.Time{time.Now()}[0],
		ToolContext:   agent.ToolContext{PendingCalls: []agent.PendingToolCall{{ID: "tc1", Name: "test"}}},
	}}

	if err := m.SaveSnapshot(context.Background(), "s1", v2); err != nil {
		t.Fatal(err)
	}
	if len(st.saved) != 1 {
		t.Fatalf("expected 1 saved snapshot, got %d", len(st.saved))
	}

	v2Target := &stateTestV2Agent{}
	snap, err := m.LoadSnapshot(context.Background(), "s1", v2Target)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ReplyID != "r1" {
		t.Fatalf("expected reply r1, got %s", snap.ReplyID)
	}
	if v2Target.state == nil || v2Target.state.ReplyID != "r1" {
		t.Fatal("expected state loaded")
	}
}

func TestSessionStateManager_HasPending(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)

	now := time.Now()
	st.saved["s1"] = &service.AgentSnapshot{
		SessionID: "s1",
		State: &agent.AgentState{
			SuspendedAt: &now,
		},
	}
	if !m.HasPendingSnapshot(context.Background(), "s1") {
		t.Fatal("expected pending")
	}
	if m.HasPendingSnapshot(context.Background(), "s2") {
		t.Fatal("expected no pending")
	}
}

func TestSessionStateManager_Resume(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)

	now := time.Now()
	st.saved["s1"] = &service.AgentSnapshot{
		SessionID: "s1",
		ReplyID:   "r1",
		State: &agent.AgentState{
			ReplyID:       "r1",
			WaitConfirmID: "c1",
			SuspendedAt:   &now,
		},
	}

	v2 := &stateTestV2Agent{state: &agent.AgentState{}}
	ev := event.NewUserConfirmResult("r1", "c1", []event.ConfirmDecision{{ToolCallID: "tc1", Decision: "allow"}})
	if err := m.Resume(context.Background(), "s1", v2, ev); err != nil {
		t.Fatal(err)
	}
	if len(v2.injected) != 1 {
		t.Fatalf("expected 1 injected event, got %d", len(v2.injected))
	}
	if _, ok := st.saved["s1"]; ok {
		t.Fatal("expected snapshot deleted after resume")
	}
}

func TestSessionStateManager_ResumeWithoutStorage(t *testing.T) {
	m := NewSessionStateManager(nil)
	v2 := &stateTestV2Agent{}
	ev := event.NewUserConfirmResult("r1", "c1", nil)
	if err := m.Resume(context.Background(), "s1", v2, ev); err != nil {
		t.Fatal(err)
	}
	if len(v2.injected) != 1 {
		t.Fatalf("expected direct inject without storage, got %d", len(v2.injected))
	}
}

func TestSessionStateManager_DeleteSnapshot(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	st.saved["s1"] = &service.AgentSnapshot{SessionID: "s1"}
	if err := m.DeleteSnapshot(context.Background(), "s1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.saved["s1"]; ok {
		t.Fatal("expected deleted")
	}
}

func TestSessionStateManager_SaveSnapshot_EmptySessionID(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	v2 := &stateTestV2Agent{state: &agent.AgentState{ReplyID: "r1"}}
	if err := m.SaveSnapshot(context.Background(), "", v2); err == nil {
		t.Fatal("expected error for empty sessionID")
	}
}

func TestSessionStateManager_LoadSnapshot_EmptySessionID(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	v2 := &stateTestV2Agent{}
	if _, err := m.LoadSnapshot(context.Background(), "", v2); err == nil {
		t.Fatal("expected error for empty sessionID")
	}
}

func TestSessionStateManager_LoadSnapshot_NilState(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	st.saved["s1"] = &service.AgentSnapshot{SessionID: "s1", State: nil}
	v2 := &stateTestV2Agent{}
	if _, err := m.LoadSnapshot(context.Background(), "s1", v2); err == nil {
		t.Fatal("expected error for nil state")
	}
}

func TestSessionStateManager_HasPendingSnapshot_EmptySessionID(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	if m.HasPendingSnapshot(context.Background(), "") {
		t.Fatal("expected false for empty sessionID")
	}
}

func TestSessionStateManager_Resume_InjectError(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	now := time.Now()
	st.saved["s1"] = &service.AgentSnapshot{
		SessionID: "s1",
		ReplyID:   "r1",
		State: &agent.AgentState{
			ReplyID:       "r1",
			WaitConfirmID: "c1",
			SuspendedAt:   &now,
		},
	}
	v2 := &stateTestV2Agent{injectErr: errors.New("inject failed")}
	ev := event.NewUserConfirmResult("r1", "c1", nil)
	if err := m.Resume(context.Background(), "s1", v2, ev); err == nil {
		t.Fatal("expected error when inject fails")
	}
}

func TestSessionStateManager_Resume_LoadError(t *testing.T) {
	st := newMockStorage()
	m := NewSessionStateManager(st)
	now := time.Now()
	st.saved["s1"] = &service.AgentSnapshot{
		SessionID: "s1",
		ReplyID:   "r1",
		State: &agent.AgentState{
			ReplyID:       "r1",
			WaitConfirmID: "c1",
			SuspendedAt:   &now,
		},
	}
	// v2.LoadState returns error, causing Resume to fail.
	v2 := &stateTestV2Agent{loadErr: errors.New("load failed")}
	ev := event.NewUserConfirmResult("r1", "c1", nil)
	if err := m.Resume(context.Background(), "s1", v2, ev); err == nil {
		t.Fatal("expected error when load fails")
	}
}
