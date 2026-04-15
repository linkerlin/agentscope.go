package session

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/redis/go-redis/v9"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *RedisSessionService) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	svc := NewRedisSessionService(client, "test:session:", time.Hour)
	return s, svc
}

func TestRedisSessionService_CreateAndGet(t *testing.T) {
	_, svc := setupMiniredis(t)

	sess, err := svc.Create("agent1")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session id")
	}

	got, err := svc.Get(sess.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.ID != sess.ID {
		t.Fatalf("expected id %s, got %s", sess.ID, got.ID)
	}
	if got.AgentName != "agent1" {
		t.Fatalf("expected agent name agent1, got %s", got.AgentName)
	}
}

func TestRedisSessionService_AddMessage(t *testing.T) {
	_, svc := setupMiniredis(t)

	sess, _ := svc.Create("agent1")
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	if err := svc.AddMessage(sess.ID, msg); err != nil {
		t.Fatalf("add message failed: %v", err)
	}

	got, err := svc.Get(sess.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].GetTextContent() != "hello" {
		t.Fatalf("expected message text 'hello', got %s", got.Messages[0].GetTextContent())
	}
}

func TestRedisSessionService_Delete(t *testing.T) {
	_, svc := setupMiniredis(t)

	sess, _ := svc.Create("agent1")
	if err := svc.Delete(sess.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err := svc.Get(sess.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRedisSessionService_List(t *testing.T) {
	_, svc := setupMiniredis(t)

	for i := 0; i < 3; i++ {
		_, _ = svc.Create("agent1")
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(list))
	}
}

func TestRedisSessionService_NotFound(t *testing.T) {
	_, svc := setupMiniredis(t)

	_, err := svc.Get("non-existent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}
