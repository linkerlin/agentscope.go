package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Job represents a scheduled agent run.
type Job struct {
	ID        string
	UserID    string
	AgentID   string
	SessionID string
	CronExpr  string
	Payload   string // user message text
	NextRun   time.Time
	Enabled   bool
}

// Scheduler manages cron-based background tasks for agent execution.
type Scheduler struct {
	cron   *cron.Cron
	jobs   map[string]cron.EntryID // jobID -> cron entryID
	mu     sync.RWMutex
	handle func(ctx context.Context, job *Job) error
}

// NewScheduler creates a new Scheduler with the given job handler.
func NewScheduler(handle func(ctx context.Context, job *Job) error) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		jobs:   make(map[string]cron.EntryID),
		handle: handle,
	}
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the cron scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// Schedule adds a new cron job.
func (s *Scheduler) Schedule(ctx context.Context, job *Job) error {
	if job.CronExpr == "" {
		return fmt.Errorf("schedule: empty cron expression")
	}
	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		_ = s.handle(ctx, job)
	})
	if err != nil {
		return fmt.Errorf("schedule: invalid cron expression: %w", err)
	}
	s.mu.Lock()
	s.jobs[job.ID] = entryID
	s.mu.Unlock()
	return nil
}

// Cancel removes a scheduled job.
func (s *Scheduler) Cancel(ctx context.Context, jobID string) error {
	s.mu.Lock()
	entryID, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("schedule: job not found: %s", jobID)
	}
	s.cron.Remove(entryID)
	delete(s.jobs, jobID)
	s.mu.Unlock()
	return nil
}

// NextRun returns the next scheduled run time for a job.
func (s *Scheduler) NextRun(jobID string) (time.Time, error) {
	s.mu.RLock()
	entryID, ok := s.jobs[jobID]
	s.mu.RUnlock()
	if !ok {
		return time.Time{}, fmt.Errorf("schedule: job not found: %s", jobID)
	}
	entry := s.cron.Entry(entryID)
	return entry.Next, nil
}
