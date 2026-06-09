package gateway

import (
	"context"

	"github.com/linkerlin/agentscope.go/schedule"
)

// scheduleManagerAdapter adapts BackgroundTaskManager to scheduletool.Manager.
type scheduleManagerAdapter struct {
	btm *BackgroundTaskManager
}

func (a scheduleManagerAdapter) Schedule(ctx context.Context, job *schedule.Job) error {
	if a.btm.storage != nil {
		return a.btm.UpsertSchedule(ctx, jobToSchedule(job, nil))
	}
	return a.btm.Schedule(ctx, job)
}

func (a scheduleManagerAdapter) Cancel(ctx context.Context, jobID string) error {
	if a.btm.storage != nil {
		sched, err := a.btm.GetSchedule(ctx, jobID)
		if err == nil {
			sched.Enabled = false
			return a.btm.UpsertSchedule(ctx, sched)
		}
	}
	return a.btm.Cancel(ctx, jobID)
}

func (a scheduleManagerAdapter) NextRun(jobID string) (string, error) {
	return a.btm.NextRunString(jobID)
}

func (a scheduleManagerAdapter) List() []*schedule.Job {
	return a.btm.List()
}

// ScheduleToolManager returns a schedule tool manager backed by this server.
func (s *Server) ScheduleToolManager() scheduleManagerAdapter {
	return scheduleManagerAdapter{btm: s.backgroundTaskMgr}
}
