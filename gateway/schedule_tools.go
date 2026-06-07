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
	return a.btm.Schedule(ctx, job)
}

func (a scheduleManagerAdapter) Cancel(ctx context.Context, jobID string) error {
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
