package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// fakeAuth is a minimal Authenticator that injects a fixed user id + role.
type fakeAuth struct{ userID, role string }

func (a fakeAuth) Authenticate(r *http.Request) (context.Context, error) {
	ctx := r.Context()
	ctx = context.WithValue(ctx, service.ContextKeyUserID, a.userID)
	ctx = context.WithValue(ctx, service.ContextKeyRole, service.Role(a.role))
	return ctx, nil
}

func TestAudit_LogsAuthenticatedRequest(t *testing.T) {
	logger := service.NewMemoryAuditLogger()
	srv := NewServer(&mockAgent{}).
		WithAuthenticator(fakeAuth{userID: "u1", role: "developer"}).
		WithAuditLogger(logger)

	hits := 0
	srv.mux.HandleFunc("GET /api/v1/ping", srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	srv.ServeHTTP(httptest.NewRecorder(), req)

	// audit is async (goroutine); wait briefly
	time.Sleep(50 * time.Millisecond)

	logs, _ := logger.ListByUser(context.Background(), "u1", 10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}
	l := logs[0]
	if l.UserID != "u1" || l.Action != "GET /api/v1/ping" || !l.Success {
		t.Fatalf("unexpected audit entry: %+v", l)
	}
	if hits != 1 {
		t.Fatal("handler should have run once")
	}
}

func TestAudit_LogsFailureStatus(t *testing.T) {
	logger := service.NewMemoryAuditLogger()
	srv := NewServer(&mockAgent{}).
		WithAuthenticator(fakeAuth{userID: "u2", role: "developer"}).
		WithAuditLogger(logger)

	srv.mux.HandleFunc("POST /api/v1/fail", srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("POST", "/api/v1/fail", nil)
	srv.ServeHTTP(httptest.NewRecorder(), req)
	time.Sleep(50 * time.Millisecond)

	logs, _ := logger.ListByUser(context.Background(), "u2", 10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Success {
		t.Fatal("500 should be logged as failure")
	}
	if logs[0].ErrorMsg == "" {
		t.Fatal("error msg should be set for failures")
	}
}

func TestAudit_NoLoggerNoOp(t *testing.T) {
	srv := NewServer(&mockAgent{}).WithAuthenticator(fakeAuth{userID: "u3"})
	called := false
	srv.mux.HandleFunc("GET /api/v1/x", srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/x", nil))
	if !called {
		t.Fatal("handler should run without audit logger")
	}
}

func TestAudit_UnauthenticatedNotLogged(t *testing.T) {
	// Auth failure (401) happens before audit, so no log entry.
	logger := service.NewMemoryAuditLogger()
	auth := failingAuth{}
	srv := NewServer(&mockAgent{}).WithAuthenticator(auth).WithAuditLogger(logger)
	srv.mux.HandleFunc("GET /api/v1/secret", srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run on auth failure")
	}))
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/secret", nil))
	time.Sleep(30 * time.Millisecond)
	logs, _ := logger.ListByUser(context.Background(), "", 10)
	if len(logs) != 0 {
		t.Fatalf("auth failure should not be audited, got %d", len(logs))
	}
}

type failingAuth struct{}

func (failingAuth) Authenticate(r *http.Request) (context.Context, error) {
	return nil, errDenied
}

var errDenied = &deniedErr{}

type deniedErr struct{}

func (*deniedErr) Error() string { return "denied" }

func TestAuditRoute_AdminCanQuery(t *testing.T) {
	logger := service.NewMemoryAuditLogger()
	logger.Log(context.Background(), &service.AuditLog{UserID: "u1", Action: "GET /x", Resource: "/x"})
	srv := NewServer(&mockAgent{}).
		WithAuthenticator(fakeAuth{userID: "admin1", role: "admin"}).
		WithAuditLogger(logger)
	srv.RegisterAuditRoutes()

	req := httptest.NewRequest("GET", "/api/v1/audit-logs?user_id=u1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin should query audit logs, got %d %s", w.Code, w.Body.String())
	}
}

func TestAuditRoute_NonAdminForbidden(t *testing.T) {
	logger := service.NewMemoryAuditLogger()
	srv := NewServer(&mockAgent{}).
		WithAuthenticator(fakeAuth{userID: "dev1", role: "developer"}).
		WithAuditLogger(logger)
	srv.RegisterAuditRoutes()

	req := httptest.NewRequest("GET", "/api/v1/audit-logs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin should be forbidden, got %d", w.Code)
	}
}
