package scheduletool

import (
	"context"
	"time"

	"github.com/linkerlin/agentscope.go/schedule"
)

// StandardManager implements Manager by wrapping a *schedule.Scheduler.
// Use this for standalone usage without the Gateway.
type StandardManager struct {
	sched *schedule.Scheduler
}

// NewStandardManager creates a StandardManager backed by the given scheduler.
func NewStandardManager(sched *schedule.Scheduler) *StandardManager {
	return &StandardManager{sched: sched}
}

func (m *StandardManager) Schedule(ctx context.Context, job *schedule.Job) error {
	return m.sched.Schedule(ctx, job)
}

func (m *StandardManager) Cancel(ctx context.Context, jobID string) error {
	return m.sched.Cancel(ctx, jobID)
}

func (m *StandardManager) NextRun(jobID string) (string, error) {
	t, err := m.sched.NextRun(jobID)
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339), nil
}

func (m *StandardManager) List() []*schedule.Job {
	return m.sched.ListJobs()
}

var _ Manager = (*StandardManager)(nil)
