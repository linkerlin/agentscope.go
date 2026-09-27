package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/service/access"
)

func ctxAsUser(userID string) context.Context {
	return context.WithValue(context.Background(), service.ContextKeyUserID, userID)
}

// TestBuildSessionAgentFromStorage_RejectsForeignAgentConfig locks the 22.2
// rule: a requester cannot run someone else's agent config by naming its ID.
// The rejection happens before credentials or workspaces are touched.
func TestBuildSessionAgentFromStorage_RejectsForeignAgentConfig(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "ag-1", UserID: "u-b", Name: "b", ModelID: "mock/m"})
	storage.SaveSession(ctx, &service.Session{ID: "s1", UserID: "u-a"})

	srv := NewServer(&mockAgent{}).WithStorage(storage)

	_, err := srv.buildSessionAgentFromStorage(ctxAsUser("u-a"), "ag-1", "s1")
	if err == nil || !strings.Contains(err.Error(), "agent config not found") {
		t.Fatalf("expected foreign-config rejection, got %v", err)
	}
}

// TestBuildSessionAgentFromStorage_OwnerPassesOwnershipGate locks the
// companion rule: the owner clears the ownership gate and proceeds into the
// pipeline (failing later on a missing credential, not on access).
func TestBuildSessionAgentFromStorage_OwnerPassesOwnershipGate(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "ag-1", UserID: "u-a", Name: "a", ModelID: "mock/m"})
	storage.SaveSession(ctx, &service.Session{ID: "s1", UserID: "u-a"})

	srv := NewServer(&mockAgent{}).WithStorage(storage)

	_, err := srv.buildSessionAgentFromStorage(ctxAsUser("u-a"), "ag-1", "s1")
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("owner should pass the gate and fail later on credentials, got %v", err)
	}
}

// TestBuildSessionAgentFromStorage_AccessPolicyGrant locks the 22.2 rule:
// cross-tenant use is possible only through an explicit Access Policy grant.
func TestBuildSessionAgentFromStorage_AccessPolicyGrant(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "ag-1", UserID: "u-b", Name: "b", ModelID: "mock/m"})
	storage.SaveSession(ctx, &service.Session{ID: "s1", UserID: "u-a"})

	// Without a policy: denied.
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	if _, err := srv.buildSessionAgentFromStorage(ctxAsUser("u-a"), "ag-1", "s1"); err == nil || !strings.Contains(err.Error(), "agent config not found") {
		t.Fatalf("expected denial without policy, got %v", err)
	}

	// With an edit grant for the viewer: the gate opens and the pipeline
	// proceeds (fails later on the missing credential).
	srv.WithAccessPolicy(&access.StaticPolicy{Refs: []access.ResourceRef{{
		Kind: access.KindAgent, OwnerID: "u-b", ResourceID: "ag-1", Permission: access.PermEdit,
	}}})
	_, err := srv.buildSessionAgentFromStorage(ctxAsUser("u-a"), "ag-1", "s1")
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("expected policy grant to open the gate, got %v", err)
	}
}
