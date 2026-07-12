package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// WithAuditLogger attaches an audit logger. When set, every authenticated
// request is recorded (who/method/path/status). nil disables auditing.
func (s *Server) WithAuditLogger(l service.AuditLogger) *Server {
	s.auditLogger = l
	return s
}

// auditWrapped is applied inside requireAuth so every protected route is
// audited automatically (zero per-handler churn). Reuses the statusRecorder
// from otel.go. When no audit logger is set it is a no-op.
func (s *Server) auditWrapped(h http.HandlerFunc) http.HandlerFunc {
	if s.auditLogger == nil {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		h(rec, r)
		s.recordAudit(r, rec.statusCode)
	}
}

func (s *Server) recordAudit(r *http.Request, status int) {
	if s.auditLogger == nil {
		return
	}
	userID := service.UserIDFromContext(r.Context())
	entry := &service.AuditLog{
		ID:        generateID("audit"),
		UserID:    userID,
		Action:    r.Method + " " + r.URL.Path,
		Resource:  r.URL.Path,
		IP:        clientIP(r),
		Success:   status < 400,
		CreatedAt: time.Now(),
	}
	if !entry.Success {
		entry.ErrorMsg = http.StatusText(status)
	}
	// best-effort, non-blocking: a failed audit log must never break the request
	go func() { _ = s.auditLogger.Log(context.Background(), entry) }()
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// RegisterAuditRoutes registers the audit-log query endpoint. Protected by
// requireAuth + an admin-role check. No-op when no audit logger is set.
func (s *Server) RegisterAuditRoutes() {
	if s.auditLogger == nil {
		return
	}
	s.mux.HandleFunc("GET /api/v1/audit-logs", s.requireAuth(s.handleListAuditLogs))
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Admin-only: viewers/developers cannot read audit logs.
	if role := service.RoleFromContext(r.Context()); role != service.RoleAdmin {
		http.Error(w, `{"error":"forbidden","message":"admin role required"}`, http.StatusForbidden)
		return
	}
	userID := r.URL.Query().Get("user_id")
	resource := r.URL.Query().Get("resource")
	limit := 100
	ctx := r.Context()
	var logs []*service.AuditLog
	var err error
	if resource != "" {
		logs, err = s.auditLogger.ListByResource(ctx, resource, limit)
	} else {
		logs, err = s.auditLogger.ListByUser(ctx, userID, limit)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_logs": logs})
}
