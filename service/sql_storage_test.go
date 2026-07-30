package service

import (
	"context"
	"testing"
	"time"
)

func newTestSQLStorage(t *testing.T) *SQLStorage {
	t.Helper()
	s, err := NewSQLStorage(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("NewSQLStorage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLStorage_UserCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	u := &User{ID: "u1", Name: "Alice", Email: "alice@test.com", APIKey: "key1"}
	if err := s.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}

	got, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Name != "Alice" || got.Email != "alice@test.com" {
		t.Fatalf("unexpected user: %+v", got)
	}

	byEmail, err := s.GetUserByEmail(ctx, "alice@test.com")
	if err != nil || byEmail.ID != "u1" {
		t.Fatalf("GetUserByEmail: %v %+v", err, byEmail)
	}

	users, err := s.ListUsers(ctx)
	if err != nil || len(users) != 1 {
		t.Fatalf("ListUsers: %v len=%d", err, len(users))
	}

	if err := s.DeleteUser(ctx, "u1"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.GetUser(ctx, "u1"); err == nil {
		t.Fatal("user should be deleted")
	}
}

func TestSQLStorage_UserUpsert(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	u := &User{ID: "u1", Name: "Alice", Email: "alice@test.com"}
	s.SaveUser(ctx, u)

	u.Name = "Alice Updated"
	s.SaveUser(ctx, u)

	got, _ := s.GetUser(ctx, "u1")
	if got.Name != "Alice Updated" {
		t.Fatalf("upsert should update name, got %s", got.Name)
	}
}

func TestSQLStorage_SessionCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	sess := &Session{ID: "s1", UserID: "u1", AgentID: "a1", Title: "Test"}
	if err := s.SaveSession(ctx, sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	got, err := s.GetSession(ctx, "s1")
	if err != nil || got.Title != "Test" {
		t.Fatalf("GetSession: %v %+v", err, got)
	}

	list, err := s.ListSessionsByUser(ctx, "u1")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSessionsByUser: %v len=%d", err, len(list))
	}

	if err := s.DeleteSession(ctx, "s1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
}

func TestSQLStorage_AgentConfigCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	cfg := &AgentConfig{ID: "a1", UserID: "u1", Name: "MyAgent", ModelID: "gpt-4"}
	if err := s.SaveAgentConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveAgentConfig: %v", err)
	}

	got, err := s.GetAgentConfig(ctx, "a1")
	if err != nil || got.Name != "MyAgent" {
		t.Fatalf("GetAgentConfig: %v %+v", err, got)
	}

	list, err := s.ListAgentConfigsByUser(ctx, "u1")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAgentConfigsByUser: %v len=%d", err, len(list))
	}

	if err := s.DeleteAgentConfig(ctx, "a1"); err != nil {
		t.Fatalf("DeleteAgentConfig: %v", err)
	}
}

func TestSQLStorage_CredentialCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	cred := &Credential{ID: "c1", UserID: "u1", Provider: "openai", Label: "OpenAI Key", Encrypted: "enc123"}
	if err := s.SaveCredential(ctx, cred); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}

	got, err := s.GetCredential(ctx, "c1")
	if err != nil || got.Provider != "openai" {
		t.Fatalf("GetCredential: %v %+v", err, got)
	}

	list, err := s.ListCredentialsByUser(ctx, "u1")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCredentialsByUser: %v len=%d", err, len(list))
	}

	if err := s.DeleteCredential(ctx, "c1"); err != nil {
		t.Fatalf("DeleteCredential: %v", err)
	}
}

func TestSQLStorage_MessageCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		msg := &StoredMessage{
			ID:        "m" + string(rune('1'+i)),
			SessionID: "s1",
			Role:      "user",
			Content:   "hello",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := s.SaveMessage(ctx, msg); err != nil {
			t.Fatalf("SaveMessage: %v", err)
		}
	}

	list, err := s.ListMessagesBySession(ctx, "s1", 3, 0)
	if err != nil || len(list) != 3 {
		t.Fatalf("ListMessagesBySession limit=3: %v len=%d", err, len(list))
	}

	all, err := s.ListMessagesBySession(ctx, "s1", 100, 0)
	if err != nil || len(all) != 5 {
		t.Fatalf("ListMessagesBySession all: %v len=%d", err, len(all))
	}

	// Upsert should replace
	msg := &StoredMessage{ID: "m1", SessionID: "s1", Role: "assistant", Content: "updated"}
	s.UpsertMessage(ctx, msg)
	got, _ := s.GetMessage(ctx, "m1")
	if got.Content != "updated" {
		t.Fatalf("upsert should update content, got %s", got.Content)
	}

	if err := s.DeleteMessagesBySession(ctx, "s1"); err != nil {
		t.Fatalf("DeleteMessagesBySession: %v", err)
	}
	all, _ = s.ListMessagesBySession(ctx, "s1", 100, 0)
	if len(all) != 0 {
		t.Fatalf("messages should be deleted, got %d", len(all))
	}
}

func TestSQLStorage_SnapshotCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	snap := &AgentSnapshot{SessionID: "s1", ReplyID: "r1"}
	if err := s.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	got, err := s.GetSnapshot(ctx, "s1")
	if err != nil || got.ReplyID != "r1" {
		t.Fatalf("GetSnapshot: %v %+v", err, got)
	}

	if err := s.DeleteSnapshot(ctx, "s1"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
}

func TestSQLStorage_ScheduleCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	sched := &Schedule{ID: "sc1", UserID: "u1", AgentID: "a1", Name: "Daily", CronExpr: "0 9 * * *", Payload: "{}", Enabled: true}
	if err := s.SaveSchedule(ctx, sched); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	got, err := s.GetSchedule(ctx, "sc1")
	if err != nil || got.Name != "Daily" {
		t.Fatalf("GetSchedule: %v %+v", err, got)
	}

	byUser, _ := s.ListSchedulesByUser(ctx, "u1")
	if len(byUser) != 1 {
		t.Fatalf("ListSchedulesByUser: len=%d", len(byUser))
	}

	all, _ := s.ListAllSchedules(ctx)
	if len(all) != 1 {
		t.Fatalf("ListAllSchedules: len=%d", len(all))
	}

	// Disabled schedule should not appear in ListAllSchedules
	sched.Enabled = false
	s.SaveSchedule(ctx, sched)
	all, _ = s.ListAllSchedules(ctx)
	if len(all) != 0 {
		t.Fatalf("disabled schedule should not be listed, got %d", len(all))
	}

	if err := s.DeleteSchedule(ctx, "sc1"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
}

func TestSQLStorage_TeamCRUD(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	team := &Team{
		ID:              "t1",
		UserID:          "u1",
		LeaderSessionID: "s_leader",
		Name:            "Team Alpha",
		Members: []TeamMember{
			{AgentID: "a1", Name: "worker1", SessionID: "s_w1"},
		},
	}
	if err := s.SaveTeam(ctx, team); err != nil {
		t.Fatalf("SaveTeam: %v", err)
	}

	got, err := s.GetTeam(ctx, "t1")
	if err != nil || got.Name != "Team Alpha" {
		t.Fatalf("GetTeam: %v %+v", err, got)
	}
	if len(got.Members) != 1 || got.Members[0].Name != "worker1" {
		t.Fatalf("team members mismatch: %+v", got.Members)
	}

	byLeader, err := s.GetTeamByLeaderSession(ctx, "s_leader")
	if err != nil || byLeader.ID != "t1" {
		t.Fatalf("GetTeamByLeaderSession: %v %+v", err, byLeader)
	}

	byUser, _ := s.ListTeamsByUser(ctx, "u1")
	if len(byUser) != 1 {
		t.Fatalf("ListTeamsByUser: len=%d", len(byUser))
	}

	if err := s.DeleteTeam(ctx, "t1"); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}
}

func TestSQLStorage_CascadeDelete(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()

	// Create user with sessions, messages, agents, credentials, teams
	s.SaveUser(ctx, &User{ID: "u1", Name: "Alice"})
	s.SaveSession(ctx, &Session{ID: "s1", UserID: "u1", AgentID: "a1"})
	s.SaveMessage(ctx, &StoredMessage{ID: "m1", SessionID: "s1", Role: "user"})
	s.SaveSnapshot(ctx, &AgentSnapshot{SessionID: "s1"})
	s.SaveAgentConfig(ctx, &AgentConfig{ID: "a1", UserID: "u1"})
	s.SaveCredential(ctx, &Credential{ID: "c1", UserID: "u1"})
	s.SaveSchedule(ctx, &Schedule{ID: "sc1", UserID: "u1", Enabled: true})
	s.SaveTeam(ctx, &Team{ID: "t1", UserID: "u1"})

	// Delete user should cascade
	if err := s.DeleteUser(ctx, "u1"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Verify cascaded
	if _, err := s.GetSession(ctx, "s1"); err == nil {
		t.Fatal("session should be cascade-deleted")
	}
	if _, err := s.GetAgentConfig(ctx, "a1"); err == nil {
		t.Fatal("agent config should be cascade-deleted")
	}
	if _, err := s.GetCredential(ctx, "c1"); err == nil {
		t.Fatal("credential should be cascade-deleted")
	}
	if _, err := s.GetSnapshot(ctx, "s1"); err == nil {
		t.Fatal("snapshot should be cascade-deleted")
	}
	msgs, _ := s.ListMessagesBySession(ctx, "s1", 10, 0)
	if len(msgs) != 0 {
		t.Fatal("messages should be cascade-deleted")
	}
}

func TestSQLStorage_ImplementsStorage(t *testing.T) {
	var _ Storage = (*SQLStorage)(nil)
}
