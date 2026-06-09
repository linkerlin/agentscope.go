package gateway

import (
	"time"

	"github.com/linkerlin/agentscope.go/schedule"
	"github.com/linkerlin/agentscope.go/service"
)

func scheduleToJob(s *service.Schedule) *schedule.Job {
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
		Enabled:    s.Enabled,
		MaxRetries: s.MaxRetries,
		RetryDelay: time.Duration(s.RetryDelayMs) * time.Millisecond,
		Timeout:    time.Duration(s.TimeoutMs) * time.Millisecond,
		LastRun:    s.LastRun,
		LastError:  s.LastError,
	}
}

func jobToSchedule(j *schedule.Job, existing *service.Schedule) *service.Schedule {
	if j == nil {
		return nil
	}
	out := existing
	if out == nil {
		out = &service.Schedule{ID: j.ID}
	}
	out.UserID = j.UserID
	out.AgentID = j.AgentID
	out.SessionID = j.SessionID
	out.CronExpr = j.CronExpr
	out.Payload = j.Payload
	out.Enabled = j.Enabled
	out.MaxRetries = j.MaxRetries
	out.RetryDelayMs = j.RetryDelay.Milliseconds()
	out.TimeoutMs = j.Timeout.Milliseconds()
	out.LastRun = j.LastRun
	out.LastError = j.LastError
	return out
}
