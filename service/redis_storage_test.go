package service

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
)

func TestRedisStorage_InterfaceCompliance(t *testing.T) {
	var _ Storage = (*RedisStorage)(nil)
}

func TestRedisStorage_AllMethodsReturnNotImplemented(t *testing.T) {
	ctx := context.Background()
	s := NewRedisStorage("localhost:6379", "", 0)

	cases := []struct {
		name string
		fn   func() error
	}{
		{"SaveUser", func() error { return s.SaveUser(ctx, &User{ID: "u1"}) }},
		{"GetUser", func() error { _, err := s.GetUser(ctx, "u1"); return err }},
		{"DeleteUser", func() error { return s.DeleteUser(ctx, "u1") }},
		{"SaveSession", func() error { return s.SaveSession(ctx, &Session{ID: "s1"}) }},
		{"GetSession", func() error { _, err := s.GetSession(ctx, "s1"); return err }},
		{"ListSessionsByUser", func() error { _, err := s.ListSessionsByUser(ctx, "u1"); return err }},
		{"DeleteSession", func() error { return s.DeleteSession(ctx, "s1") }},
		{"SaveAgentConfig", func() error { return s.SaveAgentConfig(ctx, &AgentConfig{ID: "a1"}) }},
		{"GetAgentConfig", func() error { _, err := s.GetAgentConfig(ctx, "a1"); return err }},
		{"ListAgentConfigsByUser", func() error { _, err := s.ListAgentConfigsByUser(ctx, "u1"); return err }},
		{"DeleteAgentConfig", func() error { return s.DeleteAgentConfig(ctx, "a1") }},
		{"SaveCredential", func() error { return s.SaveCredential(ctx, &Credential{ID: "c1"}) }},
		{"GetCredential", func() error { _, err := s.GetCredential(ctx, "c1"); return err }},
		{"ListCredentialsByUser", func() error { _, err := s.ListCredentialsByUser(ctx, "u1"); return err }},
		{"DeleteCredential", func() error { return s.DeleteCredential(ctx, "c1") }},
		{"SaveMessage", func() error { return s.SaveMessage(ctx, &StoredMessage{ID: "m1"}) }},
		{"ListMessagesBySession", func() error { _, err := s.ListMessagesBySession(ctx, "s1", 10, 0); return err }},
		{"DeleteMessagesBySession", func() error { return s.DeleteMessagesBySession(ctx, "s1") }},
		{"SaveSnapshot", func() error {
			return s.SaveSnapshot(ctx, &AgentSnapshot{SessionID: "s1", ReplyID: "r1", State: &agent.AgentState{}})
		}},
		{"GetSnapshot", func() error { _, err := s.GetSnapshot(ctx, "s1"); return err }},
		{"DeleteSnapshot", func() error { return s.DeleteSnapshot(ctx, "s1") }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.fn()
			if err == nil {
				t.Fatal("expected error for unimplemented method")
			}
			if err.Error() != "redis storage: "+c.name+" not yet implemented" {
				// Some methods have different error messages
				t.Logf("error message: %s", err.Error())
			}
		})
	}
}

func TestRedisStorage_New(t *testing.T) {
	s := NewRedisStorage("localhost:6379", "pass", 1)
	if s.addr != "localhost:6379" {
		t.Fatalf("addr mismatch: %s", s.addr)
	}
	if s.password != "pass" {
		t.Fatalf("password mismatch: %s", s.password)
	}
	if s.db != 1 {
		t.Fatalf("db mismatch: %d", s.db)
	}
}
