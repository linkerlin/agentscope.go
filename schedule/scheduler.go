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
	ID         string
	UserID     string
	AgentID    string
	SessionID  string
	CronExpr   string
	Payload    string // user message text
	NextRun    time.Time
	Enabled    bool
	MaxRetries int           // 0 = no retry
	RetryDelay time.Duration // delay between retries
	Timeout    time.Duration // per-run timeout; 0 = no limit
	LastRun    time.Time
	LastError  string
}

// Scheduler manages cron-based background tasks for agent execution.
type Scheduler struct {
	cron   *cron.Cron
	jobs   map[string]cron.EntryID // jobID -> cron entryID
	defs   map[string]*Job         // jobID -> job definition
	mu     sync.RWMutex
	handle func(ctx context.Context, job *Job) error
}

// NewScheduler creates a new Scheduler with the given job handler.
func NewScheduler(handle func(ctx context.Context, job *Job) error) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		jobs:   make(map[string]cron.EntryID),
		defs:   make(map[string]*Job),
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

// Schedule adds a new cron job. If a job with the same ID already exists,
// the old schedule is replaced by the new one.
func (s *Scheduler) Schedule(ctx context.Context, job *Job) error {
	if job.CronExpr == "" {
		return fmt.Errorf("schedule: empty cron expression")
	}
	s.mu.Lock()
	if oldID, ok := s.jobs[job.ID]; ok {
		s.cron.Remove(oldID)
		delete(s.jobs, job.ID)
	}
	s.mu.Unlock()

	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		_ = s.handle(ctx, job)
	})
	if err != nil {
		return fmt.Errorf("schedule: invalid cron expression: %w", err)
	}
	s.mu.Lock()
	s.jobs[job.ID] = entryID
	cp := *job
	s.defs[job.ID] = &cp
	s.mu.Unlock()
	return nil
}

// UpdateJobMeta mutates stored job metadata (e.g. last run / error).
func (s *Scheduler) UpdateJobMeta(jobID string, fn func(*Job)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.defs[jobID]
	if !ok {
		return false
	}
	fn(j)
	return true
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
	delete(s.defs, jobID)
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

// ListJobs returns all scheduled job definitions.
func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Job, 0, len(s.defs))
	for _, j := range s.defs {
		cp := *j
		out = append(out, &cp)
	}
	return out
}
