package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/schedule"
)

func TestBackgroundTaskManager_ScheduleAndCancel(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("a1", &smMockAgent{
		events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		},
	})

	btm := NewBackgroundTaskManager(reg, nil)
	btm.Start()
	defer btm.Stop()

	job := &schedule.Job{
		ID:      "j1",
		AgentID: "a1",
		CronExpr: "*/1 * * * *",
		Payload: "hello",
	}
	if err := btm.Schedule(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	next, err := btm.NextRun("j1")
	if err != nil {
		t.Fatal(err)
	}
	if next.IsZero() {
		t.Fatal("expected non-zero next run")
	}

	if err := btm.Cancel(context.Background(), "j1"); err != nil {
		t.Fatal(err)
	}

	_, err = btm.NextRun("j1")
	if err == nil {
		t.Fatal("expected error after cancel")
	}
}

func TestBackgroundTaskManager_HandleAgentNotFound(t *testing.T) {
	reg := NewAgentRegistry()
	btm := NewBackgroundTaskManager(reg, nil)

	job := &schedule.Job{
		ID:      "j1",
		AgentID: "missing",
		Payload: "hello",
	}
	err := btm.handle(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestBackgroundTaskManager_HandleWithSessionManager(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("a1", &smMockAgent{
		events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		},
		delay: 10 * time.Millisecond,
	})

	sm := NewSessionManager()
	btm := NewBackgroundTaskManager(reg, sm)

	job := &schedule.Job{
		ID:        "j1",
		AgentID:   "a1",
		SessionID: "s1",
		Payload:   "hello",
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	if sm.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions after handle, got %d", sm.ActiveCount())
	}
}

func TestBackgroundTaskManager_HandleV2Agent(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("a1", &smMockAgent{
		events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		},
	})

	btm := NewBackgroundTaskManager(reg, nil)
	job := &schedule.Job{
		ID:      "j1",
		AgentID: "a1",
		Payload: "hello",
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundTaskManager_HandleNonV2Agent(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("a1", &nonV2Agent{})

	btm := NewBackgroundTaskManager(reg, nil)
	job := &schedule.Job{
		ID:      "j1",
		AgentID: "a1",
		Payload: "hello",
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}
