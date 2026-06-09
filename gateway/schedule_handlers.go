package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/service"
)

type scheduleRequest struct {
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

type updateScheduleRequest struct {
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

type scheduleResponse struct {
	ID      string    `json:"id"`
	NextRun time.Time `json:"next_run,omitempty"`
	Error   string    `json:"error,omitempty"`
}

type listSchedulesResponse struct {
	Schedules []*service.Schedule `json:"schedules"`
	Total     int                 `json:"total"`
}

type scheduleSessionsResponse struct {
	Sessions []*service.Session `json:"sessions"`
	Total    int                `json:"total"`
}

func (s *Server) handleScheduleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleScheduleList(w, r)
	case http.MethodPost:
		s.handleScheduleCreate(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleScheduleList(w http.ResponseWriter, r *http.Request) {
	if s.backgroundTaskMgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	schedules, err := s.backgroundTaskMgr.ListSchedules(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if schedules == nil {
		schedules = []*service.Schedule{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listSchedulesResponse{Schedules: schedules, Total: len(schedules)})
}

func (s *Server) handleScheduleCreate(w http.ResponseWriter, r *http.Request) {
	if s.backgroundTaskMgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	if err := s.backgroundTaskMgr.UpsertSchedule(r.Context(), sched); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	next, _ := s.backgroundTaskMgr.NextRun(req.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(scheduleResponse{ID: req.ID, NextRun: next})
}

func (s *Server) handleScheduleItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		s.handleScheduleUpdate(w, r, id)
	case http.MethodDelete:
		s.handleScheduleDeleteByID(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleScheduleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	if s.backgroundTaskMgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	sched, err := s.backgroundTaskMgr.GetSchedule(r.Context(), id)
	if err != nil || (userID != "" && sched.UserID != userID) {
		http.Error(w, fmt.Sprintf("schedule not found: %s", id), http.StatusNotFound)
		return
	}

	var req updateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	if err := s.backgroundTaskMgr.UpsertSchedule(r.Context(), sched); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	next, _ := s.backgroundTaskMgr.NextRun(id)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(scheduleResponse{ID: id, NextRun: next})
}

func (s *Server) handleScheduleDeleteByID(w http.ResponseWriter, r *http.Request, id string) {
	if s.backgroundTaskMgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if err := s.backgroundTaskMgr.DeleteSchedule(r.Context(), userID, id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScheduleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	scheduleID := r.PathValue("id")
	userID := service.UserIDFromContext(r.Context())
	sessions, err := s.storage.ListSessionsBySchedule(r.Context(), userID, scheduleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if sessions == nil {
		sessions = []*service.Session{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(scheduleSessionsResponse{Sessions: sessions, Total: len(sessions)})
}

func (s *Server) handleScheduleDeleteLegacy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.backgroundTaskMgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if err := s.backgroundTaskMgr.DeleteSchedule(r.Context(), userID, jobID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterScheduleRoutes adds schedule CRUD endpoints aligned with PyV2 /schedule.
func (s *Server) RegisterScheduleRoutes() {
	s.mux.HandleFunc("/schedule", s.requireAuth(s.handleScheduleCollection))
	s.mux.HandleFunc("/schedule/{id}", s.requireAuth(s.handleScheduleItem))
	s.mux.HandleFunc("/schedule/{id}/sessions", s.requireAuth(s.handleScheduleSessions))
	// Legacy delete endpoint kept for backward compatibility.
	s.mux.HandleFunc("/schedule/delete", s.requireAuth(s.handleScheduleDeleteLegacy))
}
