// gateway/scheduleapi/handlers.go — the schedule CRUD HTTP face (16.2):
// same URLs, auth wrapping and behaviours as the pre-extraction root
// handlers; the root's RegisterScheduleRoutes delegates here.
package scheduleapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/service"
)

// DefaultMaxBodyBytes caps the JSON write endpoints (18.11 parity).
const DefaultMaxBodyBytes = 1 << 20

// Deps carries the handlers' narrow dependency surface.
type Deps struct {
	// BTM executes and persists schedules.
	BTM *BackgroundTaskManager
	// Storage backs the schedule-sessions listing; nil → 503 on that one
	// endpoint only.
	Storage service.Storage
	// MaxBodyBytes caps request bodies; 0 = DefaultMaxBodyBytes.
	MaxBodyBytes int64
}

// Handlers is the schedule HTTP face.
type Handlers struct {
	d Deps
}

// NewHandlers builds the handlers over the deps.
func NewHandlers(d Deps) *Handlers { return &Handlers{d: d} }

// Register mounts the schedule CRUD endpoints (URLs unchanged from the
// pre-extraction root routes). wrap is the caller's auth middleware.
func (h *Handlers) Register(mux *http.ServeMux, wrap func(http.HandlerFunc) http.HandlerFunc) {
	if wrap == nil {
		wrap = func(hf http.HandlerFunc) http.HandlerFunc { return hf }
	}
	mux.HandleFunc("/schedule", wrap(h.handleCollection))
	mux.HandleFunc("/schedule/{id}", wrap(h.handleItem))
	mux.HandleFunc("/schedule/{id}/sessions", wrap(h.handleSessions))
	// Legacy delete endpoint kept for backward compatibility.
	mux.HandleFunc("/schedule/delete", wrap(h.handleDeleteLegacy))
}

func (h *Handlers) maxBody() int64 {
	if h.d.MaxBodyBytes > 0 {
		return h.d.MaxBodyBytes
	}
	return DefaultMaxBodyBytes
}

// decodeJSONLimit decodes the body under the cap; over-limit bodies stop
// reading at the boundary (18.11 semantics).
func (h *Handlers) decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBody())
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

type ScheduleRequest struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name,omitempty"`
	Description     string `json:"description,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	AgentID         string `json:"agent_id"`
	SessionID       string `json:"session_id,omitempty"`
	CronExpr        string `json:"cron_expr"`
	Payload         string `json:"payload"`
	Enabled         *bool  `json:"enabled,omitempty"`
	MaxRetries      int    `json:"max_retries,omitempty"`
	RetryDelay      string `json:"retry_delay,omitempty"`
	Timeout         string `json:"timeout,omitempty"`
	Source          string `json:"source,omitempty"`
	SourceSessionID string `json:"source_session_id,omitempty"`
}

type UpdateScheduleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	CronExpr    *string `json:"cron_expr,omitempty"`
	Payload     *string `json:"payload,omitempty"`
	SessionID   *string `json:"session_id,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	MaxRetries  *int    `json:"max_retries,omitempty"`
	RetryDelay  *string `json:"retry_delay,omitempty"`
	Timeout     *string `json:"timeout,omitempty"`
}

type ScheduleResponse struct {
	ID      string    `json:"id"`
	NextRun time.Time `json:"next_run,omitempty"`
	Error   string    `json:"error,omitempty"`
}

type ListSchedulesResponse struct {
	Schedules []*service.Schedule `json:"schedules"`
	Total     int                 `json:"total"`
}

type ScheduleSessionsResponse struct {
	Sessions []*service.Session `json:"sessions"`
	Total    int                `json:"total"`
}

func (h *Handlers) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	if h.d.BTM == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	schedules, err := h.d.BTM.ListSchedules(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if schedules == nil {
		schedules = []*service.Schedule{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ListSchedulesResponse{Schedules: schedules, Total: len(schedules)})
}

func (h *Handlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	if h.d.BTM == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}

	var req ScheduleRequest
	if err := h.decodeJSONLimit(w, r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.AgentID == "" || req.CronExpr == "" {
		http.Error(w, "agent_id and cron_expr are required", http.StatusBadRequest)
		return
	}

	userID := service.UserIDFromContext(r.Context())
	if userID == "" {
		userID = req.UserID
	}
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	sched := &service.Schedule{
		ID:              req.ID,
		UserID:          userID,
		AgentID:         req.AgentID,
		Name:            req.Name,
		Description:     req.Description,
		CronExpr:        req.CronExpr,
		Payload:         req.Payload,
		SessionID:       req.SessionID,
		Enabled:         enabled,
		MaxRetries:      req.MaxRetries,
		Source:          req.Source,
		SourceSessionID: req.SourceSessionID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if req.RetryDelay != "" {
		if d, err := time.ParseDuration(req.RetryDelay); err == nil {
			sched.RetryDelayMs = d.Milliseconds()
		}
	}
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			sched.TimeoutMs = d.Milliseconds()
		}
	}
	if sched.Source == "" {
		sched.Source = "USER"
	}

	if err := h.d.BTM.UpsertSchedule(r.Context(), sched); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	next, _ := h.d.BTM.NextRun(req.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ScheduleResponse{ID: req.ID, NextRun: next})
}

func (h *Handlers) handleItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		h.handleUpdate(w, r, id)
	case http.MethodDelete:
		h.handleDeleteByID(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	if h.d.BTM == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	sched, err := h.d.BTM.GetSchedule(r.Context(), id)
	if err != nil || (userID != "" && sched.UserID != userID) {
		http.Error(w, fmt.Sprintf("schedule not found: %s", id), http.StatusNotFound)
		return
	}

	var req UpdateScheduleRequest
	if err := h.decodeJSONLimit(w, r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name != nil {
		sched.Name = *req.Name
	}
	if req.Description != nil {
		sched.Description = *req.Description
	}
	if req.CronExpr != nil {
		sched.CronExpr = *req.CronExpr
	}
	if req.Payload != nil {
		sched.Payload = *req.Payload
	}
	if req.SessionID != nil {
		sched.SessionID = *req.SessionID
	}
	if req.Enabled != nil {
		sched.Enabled = *req.Enabled
	}
	if req.MaxRetries != nil {
		sched.MaxRetries = *req.MaxRetries
	}
	if req.RetryDelay != nil {
		if d, err := time.ParseDuration(*req.RetryDelay); err == nil {
			sched.RetryDelayMs = d.Milliseconds()
		}
	}
	if req.Timeout != nil {
		if d, err := time.ParseDuration(*req.Timeout); err == nil {
			sched.TimeoutMs = d.Milliseconds()
		}
	}
	sched.UpdatedAt = time.Now()

	if err := h.d.BTM.UpsertSchedule(r.Context(), sched); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	next, _ := h.d.BTM.NextRun(id)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ScheduleResponse{ID: id, NextRun: next})
}

func (h *Handlers) handleDeleteByID(w http.ResponseWriter, r *http.Request, id string) {
	if h.d.BTM == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if err := h.d.BTM.DeleteSchedule(r.Context(), userID, id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.d.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	scheduleID := r.PathValue("id")
	userID := service.UserIDFromContext(r.Context())
	sessions, err := h.d.Storage.ListSessionsBySchedule(r.Context(), userID, scheduleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if sessions == nil {
		sessions = []*service.Session{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ScheduleSessionsResponse{Sessions: sessions, Total: len(sessions)})
}

func (h *Handlers) handleDeleteLegacy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.d.BTM == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if err := h.d.BTM.DeleteSchedule(r.Context(), userID, jobID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
