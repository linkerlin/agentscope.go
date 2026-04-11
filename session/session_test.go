package session

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestSessionService(t *testing.T) {
	svc := NewInMemorySessionService()

	sess, err := svc.Create("agent1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.AgentName != "agent1" {
		t.Errorf("expected agent1, got %s", sess.AgentName)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	if err := svc.AddMessage(sess.ID, msg); err != nil {
		t.Fatal(err)
	}

	fetched, err := svc.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(fetched.Messages))
	}

	list, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}

	if err := svc.Delete(sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(sess.ID); err == nil {
		t.Error("expected error after delete")
	}
}
