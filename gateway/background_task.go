package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/schedule"
)

// BackgroundTaskManager wires the schedule.Scheduler to the AgentRegistry
// and SessionManager so that cron-triggered jobs actually invoke agents.
type BackgroundTaskManager struct {
	scheduler   *schedule.Scheduler
	registry    *AgentRegistry
	sessionMgr  *SessionManager
	toolOffload *ToolOffloadManager
}

// NewBackgroundTaskManager creates a manager and starts the internal cron
// scheduler. Call Stop() on shutdown.
func NewBackgroundTaskManager(registry *AgentRegistry, sessionMgr *SessionManager) *BackgroundTaskManager {
	btm := &BackgroundTaskManager{
		registry:   registry,
		sessionMgr: sessionMgr,
	}
	btm.scheduler = schedule.NewScheduler(btm.handle)
	return btm
}

// ToolOffload returns the tool offload manager (lazy init).
func (btm *BackgroundTaskManager) ToolOffload() *ToolOffloadManager {
	if btm.toolOffload == nil {
		btm.toolOffload = NewToolOffloadManager()
	}
	return btm.toolOffload
}

// Start begins the cron scheduler.
func (btm *BackgroundTaskManager) Start() {
	btm.scheduler.Start()
}

// Stop halts the cron scheduler.
func (btm *BackgroundTaskManager) Stop() {
	btm.scheduler.Stop()
}

// Schedule adds or replaces a cron job.
func (btm *BackgroundTaskManager) Schedule(ctx context.Context, job *schedule.Job) error {
	return btm.scheduler.Schedule(ctx, job)
}

// Cancel removes a scheduled job.
func (btm *BackgroundTaskManager) Cancel(ctx context.Context, jobID string) error {
	return btm.scheduler.Cancel(ctx, jobID)
}

// NextRun returns the next scheduled execution time for a job.
func (btm *BackgroundTaskManager) NextRun(jobID string) (time.Time, error) {
	return btm.scheduler.NextRun(jobID)
}

// List returns all scheduled jobs.
func (btm *BackgroundTaskManager) List() []*schedule.Job {
	if btm.scheduler == nil {
		return nil
	}
	return btm.scheduler.ListJobs()
}

// NextRunString returns the next run time as RFC3339 text.
func (btm *BackgroundTaskManager) NextRunString(jobID string) (string, error) {
	t, err := btm.NextRun(jobID)
	if err != nil {
		return "", err
	}
	if t.IsZero() {
		return "", nil
	}
	return t.Format(time.RFC3339), nil
}
func (btm *BackgroundTaskManager) handle(ctx context.Context, job *schedule.Job) error {
	var lastErr error
	attempts := job.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		runCtx := ctx
		var cancel context.CancelFunc
		if job.Timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, job.Timeout)
		}
		lastErr = btm.runOnce(runCtx, job)
		if cancel != nil {
			cancel()
		}
		if lastErr == nil {
			btm.setJobStatus(job.ID, "", time.Now())
			return nil
		}
		if i+1 < attempts && job.RetryDelay > 0 {
			select {
			case <-time.After(job.RetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	btm.setJobStatus(job.ID, lastErr.Error(), time.Now())
	return lastErr
}

func (btm *BackgroundTaskManager) setJobStatus(jobID, errMsg string, lastRun time.Time) {
	if btm.scheduler == nil {
		return
	}
	_ = btm.scheduler.UpdateJobMeta(jobID, func(j *schedule.Job) {
		j.LastError = errMsg
		j.LastRun = lastRun
	})
}

func (btm *BackgroundTaskManager) runOnce(ctx context.Context, job *schedule.Job) error {
	a, err := btm.registry.Get(ctx, job.AgentID)
	if err != nil {
		return fmt.Errorf("background_task: resolve agent %q: %w", job.AgentID, err)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent(job.Payload).Build()

	if btm.sessionMgr != nil && job.SessionID != "" {
		ch, err := btm.sessionMgr.Run(ctx, job.SessionID, a, msg)
		if err != nil {
			return fmt.Errorf("background_task: session run: %w", err)
		}
		for range ch {
		} // drain until completion
		return nil
	}

	if v2, ok := a.(agent.V2Agent); ok {
		ch, err := v2.ReplyStream(ctx, msg)
		if err != nil {
			return fmt.Errorf("background_task: reply stream: %w", err)
		}
		for range ch {
		} // drain until completion
		return nil
	}

	_, err = a.Call(ctx, msg)
	return err
}

// scheduleRequest is the JSON body for creating a scheduled job.
type scheduleRequest struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	AgentID    string        `json:"agent_id"`
	SessionID  string        `json:"session_id,omitempty"`
	CronExpr   string        `json:"cron_expr"`
	Payload    string        `json:"payload"`
	MaxRetries int           `json:"max_retries,omitempty"`
	RetryDelay string        `json:"retry_delay,omitempty"` // Go duration, e.g. "5s"
	Timeout    string        `json:"timeout,omitempty"`
}

// scheduleResponse is the JSON response for a scheduled job.
type scheduleResponse struct {
	ID      string    `json:"id"`
	NextRun time.Time `json:"next_run,omitempty"`
	Error   string    `json:"error,omitempty"`
}

func (s *Server) handleScheduleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.backgroundTaskMgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.AgentID == "" || req.CronExpr == "" {
		http.Error(w, "id, agent_id and cron_expr are required", http.StatusBadRequest)
		return
	}

	job := &schedule.Job{
		ID:         req.ID,
		UserID:     req.UserID,
		AgentID:    req.AgentID,
		SessionID:  req.SessionID,
		CronExpr:   req.CronExpr,
		Payload:    req.Payload,
		Enabled:    true,
		MaxRetries: req.MaxRetries,
	}
	if req.RetryDelay != "" {
		if d, err := time.ParseDuration(req.RetryDelay); err == nil {
			job.RetryDelay = d
		}
	}
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			job.Timeout = d
		}
	}
	if err := s.backgroundTaskMgr.Schedule(r.Context(), job); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	next, _ := s.backgroundTaskMgr.NextRun(req.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(scheduleResponse{ID: req.ID, NextRun: next})
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
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
	if err := s.backgroundTaskMgr.Cancel(r.Context(), jobID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterScheduleRoutes adds the background-task schedule endpoints.
func (s *Server) RegisterScheduleRoutes() {
	s.mux.HandleFunc("/schedule", s.requireAuth(s.handleScheduleCreate))
	s.mux.HandleFunc("/schedule/delete", s.requireAuth(s.handleScheduleDelete))
}
