package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/linkerlin/agentscope.go/agent"
)

func setupRedisStorage(t *testing.T) (*RedisStorage, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := NewRedisStorage(client)
	return s, mr
}

func TestRedisStorage_InterfaceCompliance(t *testing.T) {
	var _ Storage = (*RedisStorage)(nil)
}

func TestRedisStorage_New(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	s := NewRedisStorageFromAddr(mr.Addr(), "", 0)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestRedisStorage_UserCRUD(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	u := &User{ID: "u1", Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()}
	if err := s.SaveUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Alice" {
		t.Fatalf("name mismatch: %s", got.Name)
	}

	if err := s.DeleteUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetUser(ctx, "u1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRedisStorage_SessionCRUD(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	se := &Session{ID: "s1", UserID: "u1", AgentID: "a1", Title: "test", CreatedAt: time.Now()}
	if err := s.SaveSession(ctx, se); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetSession(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "test" {
		t.Fatalf("title mismatch: %s", got.Title)
	}

	sessions, err := s.ListSessionsByUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if err := s.DeleteSession(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession(ctx, "s1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRedisStorage_AgentConfigCRUD(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	cfg := &AgentConfig{ID: "a1", UserID: "u1", Name: "agent1", ModelID: "gpt-4", CreatedAt: time.Now()}
	if err := s.SaveAgentConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetAgentConfig(ctx, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "agent1" {
		t.Fatalf("name mismatch: %s", got.Name)
	}

	cfgs, err := s.ListAgentConfigsByUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(cfgs))
	}

	if err := s.DeleteAgentConfig(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
}

func TestRedisStorage_CredentialCRUD(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	c := &Credential{ID: "c1", UserID: "u1", Provider: "openai", Label: "prod", Encrypted: "enc", CreatedAt: time.Now()}
	if err := s.SaveCredential(ctx, c); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCredential(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "openai" {
		t.Fatalf("provider mismatch: %s", got.Provider)
	}

	creds, err := s.ListCredentialsByUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}

	if err := s.DeleteCredential(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
}

func TestRedisStorage_MessagePagination(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	for i := 0; i < 10; i++ {
		m := &StoredMessage{ID: fmt.Sprintf("m%d", i), SessionID: "s1", Role: "user", Content: fmt.Sprintf("msg%d", i), CreatedAt: time.Now()}
		if err := s.SaveMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	msgs, err := s.ListMessagesBySession(ctx, "s1", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	msgs2, _ := s.ListMessagesBySession(ctx, "s1", 5, 5)
	if len(msgs2) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs2))
	}

	msgs3, _ := s.ListMessagesBySession(ctx, "s1", 5, 20)
	if len(msgs3) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs3))
	}

	if err := s.DeleteMessagesBySession(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	msgs4, _ := s.ListMessagesBySession(ctx, "s1", 100, 0)
	if len(msgs4) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(msgs4))
	}
}

func TestRedisStorage_SnapshotCRUD(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	snap := &AgentSnapshot{
		SessionID: "s1",
		ReplyID:   "r1",
		State: &agent.AgentState{
			ReplyID: "r1",
			CurIter: 2,
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveSnapshot(ctx, snap); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetSnapshot(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ReplyID != "r1" {
		t.Fatalf("reply_id mismatch: %s", got.ReplyID)
	}
	if got.State == nil || got.State.CurIter != 2 {
		t.Fatal("state mismatch")
	}

	if err := s.DeleteSnapshot(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSnapshot(ctx, "s1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRedisStorage_DeleteSession_Cascades(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	_ = s.SaveSession(ctx, &Session{ID: "s1", UserID: "u1", CreatedAt: time.Now()})
	_ = s.SaveMessage(ctx, &StoredMessage{ID: "m1", SessionID: "s1", Role: "user", Content: "hi", CreatedAt: time.Now()})
	_ = s.SaveSnapshot(ctx, &AgentSnapshot{SessionID: "s1", ReplyID: "r1", CreatedAt: time.Now()})

	_ = s.DeleteSession(ctx, "s1")

	if _, err := s.GetSession(ctx, "s1"); err == nil {
		t.Fatal("expected session deleted")
	}
	msgs, _ := s.ListMessagesBySession(ctx, "s1", 100, 0)
	if len(msgs) != 0 {
		t.Fatal("expected messages deleted")
	}
	if _, err := s.GetSnapshot(ctx, "s1"); err == nil {
		t.Fatal("expected snapshot deleted")
	}
}

func TestRedisStorage_GetMessage(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	m := &StoredMessage{ID: "m1", SessionID: "s1", Role: "user", Content: "hi", CreatedAt: time.Now()}
	_ = s.SaveMessage(ctx, m)

	got, err := s.GetMessage(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "m1" {
		t.Fatalf("id mismatch: %s", got.ID)
	}

	if _, err := s.GetMessage(ctx, "notfound"); err == nil {
		t.Fatal("expected error for missing message")
	}
}

func TestRedisStorage_UpsertMessage_Replace(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	m := &StoredMessage{ID: "m1", SessionID: "s1", Role: "user", Content: "v1", CreatedAt: time.Now()}
	_ = s.SaveMessage(ctx, m)

	updated := &StoredMessage{ID: "m1", SessionID: "s1", Role: "assistant", Content: "v2", CreatedAt: time.Now()}
	if err := s.UpsertMessage(ctx, updated); err != nil {
		t.Fatal(err)
	}

	msgs, _ := s.ListMessagesBySession(ctx, "s1", 10, 0)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "v2" {
		t.Fatalf("expected updated content v2, got %s", msgs[0].Content)
	}
}

func TestRedisStorage_UpsertMessage_Append(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	m1 := &StoredMessage{ID: "m1", SessionID: "s1", Role: "user", Content: "v1", CreatedAt: time.Now()}
	_ = s.SaveMessage(ctx, m1)

	m2 := &StoredMessage{ID: "m2", SessionID: "s1", Role: "assistant", Content: "v2", CreatedAt: time.Now()}
	if err := s.UpsertMessage(ctx, m2); err != nil {
		t.Fatal(err)
	}

	msgs, _ := s.ListMessagesBySession(ctx, "s1", 10, 0)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestRedisStorage_ListSessionsByUser_Empty(t *testing.T) {
	ctx := context.Background()
	s, mr := setupRedisStorage(t)
	defer mr.Close()

	sessions, err := s.ListSessionsByUser(ctx, "nobody")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}
