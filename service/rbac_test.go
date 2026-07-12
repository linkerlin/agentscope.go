package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHasPermission_RoleMatrix(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleAdmin, PermSystemAdmin, true},
		{RoleAdmin, PermAgentDelete, true},
		{RoleDeveloper, PermAgentWrite, true},
		{RoleDeveloper, PermAgentDelete, false}, // developer cannot delete
		{RoleDeveloper, PermSystemAdmin, false},
		{RoleViewer, PermAgentRead, true},
		{RoleViewer, PermAgentWrite, false},
		{RoleViewer, PermCredentialWrite, false},
		{Role("unknown"), PermAgentRead, false}, // unknown role → nothing
	}
	for _, c := range cases {
		if got := HasPermission(c.role, c.perm); got != c.want {
			t.Errorf("HasPermission(%s,%s)=%v want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestHasAnyPermission(t *testing.T) {
	if !HasAnyPermission(RoleDeveloper, PermAgentRead, PermAgentDelete) {
		t.Fatal("developer has agent:read, so HasAny should be true")
	}
	if HasAnyPermission(RoleViewer, PermAgentDelete, PermSystemAdmin) {
		t.Fatal("viewer has none of these")
	}
}

func TestRBACMiddleware_AllowsAuthorized(t *testing.T) {
	h := RBACMiddleware(PermAgentWrite)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), ContextKeyRole, RoleDeveloper)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRBACMiddleware_DeniesUnauthorized(t *testing.T) {
	h := RBACMiddleware(PermAgentDelete)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))
	req := httptest.NewRequest("DELETE", "/", nil)
	ctx := context.WithValue(req.Context(), ContextKeyRole, RoleViewer)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRBACMiddleware_DefaultViewer(t *testing.T) {
	// No role in context → defaults to viewer → read allowed, write denied.
	readH := RBACMiddleware(PermAgentRead)(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	readH.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("viewer read should be allowed, got %d", w.Code)
	}
	writeH := RBACMiddleware(PermAgentWrite)(okHandler())
	w2 := httptest.NewRecorder()
	writeH.ServeHTTP(w2, req)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("viewer write should be denied, got %d", w2.Code)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestRoleFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKeyRole, RoleAdmin)
	if RoleFromContext(ctx) != RoleAdmin {
		t.Fatal("should return admin")
	}
	if RoleFromContext(context.Background()) != RoleViewer {
		t.Fatal("missing role should default to viewer")
	}
}

func TestVerifyRoleAssignment(t *testing.T) {
	cases := []struct {
		current, target Role
		wantErr         bool
	}{
		{RoleAdmin, RoleAdmin, false},      // admin can assign admin
		{RoleDeveloper, RoleAdmin, true},   // developer cannot assign admin
		{RoleDeveloper, RoleViewer, false}, // can assign same-or-lower
		{RoleViewer, RoleDeveloper, true},  // cannot assign higher
		{RoleViewer, RoleAdmin, true},
	}
	for _, c := range cases {
		err := VerifyRoleAssignment(c.current, c.target)
		if c.wantErr && err == nil {
			t.Errorf("VerifyRoleAssignment(%s→%s) expected error", c.current, c.target)
		}
		if !c.wantErr && err != nil {
			t.Errorf("VerifyRoleAssignment(%s→%s) unexpected error: %v", c.current, c.target, err)
		}
	}
}

func TestMemoryAuditLogger_CRUD(t *testing.T) {
	l := NewMemoryAuditLogger()
	ctx := context.Background()
	l.Log(ctx, &AuditLog{UserID: "u1", Action: "agent:create", Resource: "agent:a1", Success: true})
	l.Log(ctx, &AuditLog{UserID: "u1", Action: "tool:execute", Resource: "tool:bash", Success: true})
	l.Log(ctx, &AuditLog{UserID: "u2", Action: "agent:create", Resource: "agent:a2", Success: true})

	byUser, _ := l.ListByUser(ctx, "u1", 10)
	if len(byUser) != 2 {
		t.Fatalf("expected 2 logs for u1, got %d", len(byUser))
	}
	// most-recent-first
	if byUser[0].Action != "tool:execute" {
		t.Fatalf("expected most-recent first, got %q", byUser[0].Action)
	}

	byRes, _ := l.ListByResource(ctx, "agent:", 10)
	if len(byRes) != 2 {
		t.Fatalf("expected 2 agent: logs, got %d", len(byRes))
	}
}

func TestMemoryAuditLogger_Empty(t *testing.T) {
	l := NewMemoryAuditLogger()
	byUser, _ := l.ListByUser(context.Background(), "nobody", 10)
	if len(byUser) != 0 {
		t.Fatal("expected empty")
	}
}

func TestUserRole_OrgField(t *testing.T) {
	// UserRole carries OrgID for org/workspace isolation.
	ur := UserRole{UserID: "u1", Role: RoleDeveloper, OrgID: "org-acme"}
	if ur.OrgID != "org-acme" {
		t.Fatal("org id not set")
	}
}
