package scheduletool

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/schedule"
)

func TestStandardManager_ImplementsManager(t *testing.T) {
	var _ Manager = (*StandardManager)(nil)
}

func TestStandardManager_ScheduleAndList(t *testing.T) {
	var fired []string
	handle := func(ctx context.Context, job *schedule.Job) error {
		fired = append(fired, job.ID)
		return nil
	}
	sched := schedule.NewScheduler(handle)
	sched.Start()
	defer sched.Stop()

	mgr := NewStandardManager(sched)
	ctx := context.Background()

	err := mgr.Schedule(ctx, &schedule.Job{ID: "j1", AgentID: "a1", CronExpr: "@every 1h", Payload: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	jobs := mgr.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	next, err := mgr.NextRun("j1")
	if err != nil {
		t.Fatal(err)
	}
	if next == "" {
		t.Fatal("expected non-empty next run")
	}

	err = mgr.Cancel(ctx, "j1")
	if err != nil {
		t.Fatal(err)
	}

	if len(mgr.List()) != 0 {
		t.Fatal("expected 0 jobs after cancel")
	}
}

func TestStandardManager_CancelNotFound(t *testing.T) {
	handle := func(ctx context.Context, job *schedule.Job) error { return nil }
	sched := schedule.NewScheduler(handle)
	mgr := NewStandardManager(sched)

	err := mgr.Cancel(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestStandardManager_NextRunNotFound(t *testing.T) {
	handle := func(ctx context.Context, job *schedule.Job) error { return nil }
	sched := schedule.NewScheduler(handle)
	mgr := NewStandardManager(sched)

	_, err := mgr.NextRun("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestStandardManager_IntegrationWithTools(t *testing.T) {
	handle := func(ctx context.Context, job *schedule.Job) error { return nil }
	sched := schedule.NewScheduler(handle)
	sched.Start()
	defer sched.Stop()

	mgr := NewStandardManager(sched)
	tools := RegisterTools(mgr)

	create := tools[0]
	resp, err := create.Execute(context.Background(), map[string]any{
		"id": "int-1", "agent_id": "a1", "cron_expr": "@daily", "payload": "run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected response text")
	}

	list := tools[1]
	resp, err = list.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected non-empty list response")
	}
}
