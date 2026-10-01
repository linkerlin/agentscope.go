// Package scheduleapi is the 16.2 third registration-functionized
// cluster: the cron/schedule engine (BackgroundTaskManager) and its HTTP
// CRUD face. Dependencies are narrowed to small interfaces (Agents, Sessions)
// plus service.Storage so the cluster does not import the gateway root; the
// root keeps a thin assembly wrapper (see gateway/schedule_compat.go).
package scheduleapi

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/schedule"
	"github.com/linkerlin/agentscope.go/service"
)

// Agents resolves an agent by id (satisfied structurally by the gateway
// root's *AgentRegistry).
type Agents interface {
	Get(ctx context.Context, id string) (agent.Agent, error)
}

// Sessions starts a session turn (satisfied structurally by the gateway
// root's *SessionManager).
type Sessions interface {
	Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)
}

// RunFunc starts a session turn, usually through the coordinator (18.5).
type RunFunc func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)

// BackgroundTaskManager wires the schedule.Scheduler to the agent registry
// and session manager so that cron-triggered jobs actually invoke agents.
type BackgroundTaskManager struct {
	scheduler *schedule.Scheduler
	registry  Agents
	sessions  Sessions
	storage   service.Storage
	// sessionRun starts cron-triggered turns; wired to the coordinator entry
	// by the assembler (18.5) so a job firing on two worker replicas runs on
	// at most one (same session → same lease). nil falls back to sessions.Run.
	sessionRun RunFunc
}

// NewBackgroundTaskManager creates a manager and starts the internal cron
// scheduler. Call Stop() on shutdown.
func NewBackgroundTaskManager(registry Agents, sessions Sessions) *BackgroundTaskManager {
	btm := &BackgroundTaskManager{
		registry: registry,
		sessions: sessions,
	}
	btm.scheduler = schedule.NewScheduler(btm.handle)
	return btm
}

// WithStorage enables schedule persistence and session linkage.
func (btm *BackgroundTaskManager) WithStorage(st service.Storage) *BackgroundTaskManager {
	btm.storage = st
	return btm
}

// WithSessionRun routes cron-triggered turns through fn (typically the
// SessionCoordinator entry, 18.5).
func (btm *BackgroundTaskManager) WithSessionRun(fn RunFunc) *BackgroundTaskManager {
	btm.sessionRun = fn
	return btm
}

// Storage exposes the wired storage (nil when persistence is off) — used by
// the root's tool adapters, which need the same persistence signal.
func (btm *BackgroundTaskManager) Storage() service.Storage { return btm.storage }

// Start loads persisted schedules and begins the cron scheduler.
func (btm *BackgroundTaskManager) Start() {
	btm.loadPersistedSchedules(context.Background())
	btm.scheduler.Start()
}

func (btm *BackgroundTaskManager) loadPersistedSchedules(ctx context.Context) {
	if btm.storage == nil {
		return
	}
	schedules, err := btm.storage.ListAllSchedules(ctx)
	if err != nil {
		return
	}
	for _, s := range schedules {
		if !s.Enabled {
			continue
		}
		_ = btm.scheduler.Schedule(ctx, ScheduleToJob(s))
	}
}

// Stop halts the cron scheduler.
func (btm *BackgroundTaskManager) Stop() {
	btm.scheduler.Stop()
}

// Schedule adds or replaces a cron job in memory only.
func (btm *BackgroundTaskManager) Schedule(ctx context.Context, job *schedule.Job) error {
	return btm.scheduler.Schedule(ctx, job)
}

// UpsertSchedule persists and registers a schedule.
func (btm *BackgroundTaskManager) UpsertSchedule(ctx context.Context, sched *service.Schedule) error {
	if sched == nil {
		return fmt.Errorf("schedule: nil record")
	}
	if btm.storage != nil {
		if err := btm.storage.SaveSchedule(ctx, sched); err != nil {
			return err
		}
	}
	if !sched.Enabled {
		_ = btm.scheduler.Cancel(ctx, sched.ID)
		if btm.storage != nil {
			return btm.storage.SaveSchedule(ctx, sched)
		}
		return nil
	}
	return btm.scheduler.Schedule(ctx, ScheduleToJob(sched))
}

// GetSchedule returns a schedule by ID from storage or the in-memory scheduler.
func (btm *BackgroundTaskManager) GetSchedule(ctx context.Context, id string) (*service.Schedule, error) {
	if btm.storage != nil {
		return btm.storage.GetSchedule(ctx, id)
	}
	for _, j := range btm.List() {
		if j.ID == id {
			return JobToSchedule(j, nil), nil
		}
	}
	return nil, fmt.Errorf("schedule not found: %s", id)
}

// ListSchedules returns schedules for a user.
func (btm *BackgroundTaskManager) ListSchedules(ctx context.Context, userID string) ([]*service.Schedule, error) {
	if btm.storage == nil {
		jobs := btm.List()
		out := make([]*service.Schedule, 0, len(jobs))
		for _, j := range jobs {
			if userID == "" || j.UserID == userID {
				out = append(out, JobToSchedule(j, nil))
			}
		}
		return out, nil
	}
	return btm.storage.ListSchedulesByUser(ctx, userID)
}

// Cancel removes a scheduled job from the in-memory cron scheduler.
func (btm *BackgroundTaskManager) Cancel(ctx context.Context, jobID string) error {
	return btm.scheduler.Cancel(ctx, jobID)
}

// DeleteSchedule removes a schedule and its linked sessions for the owner.
func (btm *BackgroundTaskManager) DeleteSchedule(ctx context.Context, userID, scheduleID string) error {
	if btm.storage != nil {
		sched, err := btm.storage.GetSchedule(ctx, scheduleID)
		if err != nil {
			return err
		}
		if sched.UserID != userID {
			return fmt.Errorf("schedule not found: %s", scheduleID)
		}
		sessions, _ := btm.storage.ListSessionsBySchedule(ctx, userID, scheduleID)
		for _, se := range sessions {
			_ = btm.storage.DeleteSession(ctx, se.ID)
		}
		_ = btm.scheduler.Cancel(ctx, scheduleID)
		return btm.storage.DeleteSchedule(ctx, scheduleID)
	}
	return btm.Cancel(ctx, scheduleID)
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
	if btm.storage == nil {
		return
	}
	sched, err := btm.storage.GetSchedule(context.Background(), jobID)
	if err != nil {
		return
	}
	sched.LastError = errMsg
	sched.LastRun = lastRun
	_ = btm.storage.SaveSchedule(context.Background(), sched)
}

func (btm *BackgroundTaskManager) runOnce(ctx context.Context, job *schedule.Job) error {
	a, err := btm.registry.Get(ctx, job.AgentID)
	if err != nil {
		return fmt.Errorf("background_task: resolve agent %q: %w", job.AgentID, err)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent(job.Payload).Build()

	sessionID := job.SessionID
	if sessionID == "" && btm.storage != nil {
		sessionID = uuid.New().String()
		se := &service.Session{
			ID:               sessionID,
			UserID:           job.UserID,
			AgentID:          job.AgentID,
			Title:            "schedule:" + job.ID,
			SourceScheduleID: job.ID,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		_ = btm.storage.SaveSession(ctx, se)
	}

	if btm.sessions != nil && sessionID != "" {
		// Turns go through the coordinator when wired (18.5): a job firing
		// on two worker replicas contends on the same session lease and only
		// one executes; the loser reports busy and the retry policy decides.
		run := btm.sessionRun
		if run == nil {
			run = btm.sessions.Run
		}
		ch, err := run(ctx, sessionID, a, msg)
		if err != nil {
			return fmt.Errorf("background_task: session run: %w", err)
		}
		for range ch {
		} // drain until completion
		return nil
	}

	if v2, ok := a.(interface {
		ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error)
	}); ok {
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

// --- schedule ↔ job conversion (moved from the gateway root verbatim) ---

// ScheduleToJob converts a persisted schedule into a cron job entry.
func ScheduleToJob(s *service.Schedule) *schedule.Job {
	if s == nil {
		return nil
	}
	return &schedule.Job{
		ID:         s.ID,
		UserID:     s.UserID,
		AgentID:    s.AgentID,
		SessionID:  s.SessionID,
		CronExpr:   s.CronExpr,
		Payload:    s.Payload,
		MaxRetries: s.MaxRetries,
		RetryDelay: time.Duration(s.RetryDelayMs) * time.Millisecond,
		Timeout:    time.Duration(s.TimeoutMs) * time.Millisecond,
	}
}

// JobToSchedule merges a cron job into a persisted schedule (existing nil
// means fresh create).
func JobToSchedule(j *schedule.Job, existing *service.Schedule) *service.Schedule {
	s := existing
	if s == nil {
		s = &service.Schedule{}
	}
	s.ID = j.ID
	s.UserID = j.UserID
	s.AgentID = j.AgentID
	s.SessionID = j.SessionID
	s.CronExpr = j.CronExpr
	s.Payload = j.Payload
	s.MaxRetries = j.MaxRetries
	s.RetryDelayMs = j.RetryDelay.Milliseconds()
	s.TimeoutMs = j.Timeout.Milliseconds()
	return s
}
