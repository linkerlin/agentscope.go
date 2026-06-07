package gateway

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
)

func TestAgentRegistry_RegisterAndGet(t *testing.T) {
	r := NewAgentRegistry()
	mock := &smMockAgent{}
	r.Register("a1", mock)

	a, err := r.Get(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	if a != mock {
		t.Fatal("expected same agent instance")
	}
}

func TestAgentRegistry_GetNotFound(t *testing.T) {
	r := NewAgentRegistry()
	_, err := r.Get(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestAgentRegistry_Remove(t *testing.T) {
	r := NewAgentRegistry()
	mock := &smMockAgent{}
	r.Register("a1", mock)
	r.Remove("a1")

	_, err := r.Get(context.Background(), "a1")
	if err == nil {
		t.Fatal("expected error after removal")
	}
}

func TestAgentRegistry_Len(t *testing.T) {
	r := NewAgentRegistry()
	if r.Len() != 0 {
		t.Fatalf("expected 0, got %d", r.Len())
	}
	r.Register("a1", &smMockAgent{})
	if r.Len() != 1 {
		t.Fatalf("expected 1, got %d", r.Len())
	}
}

func TestAgentRegistry_GetReturnsV2(t *testing.T) {
	r := NewAgentRegistry()
	mock := &smMockAgent{}
	r.Register("a1", mock)

	a, err := r.Get(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	_, ok := a.(agent.V2Agent)
	if !ok {
		t.Fatal("expected V2Agent")
	}
}
