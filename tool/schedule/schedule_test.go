package scheduletool

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/schedule"
	"github.com/linkerlin/agentscope.go/tool"
)

type mockManager struct {
	jobs    map[string]*schedule.Job
	nextRun map[string]time.Time

	// error injection
	scheduleErr map[string]error
	cancelErr   map[string]error
}

func newMockManager() *mockManager {
	return &mockManager{
		jobs:        make(map[string]*schedule.Job),
		nextRun:     make(map[string]time.Time),
		scheduleErr: make(map[string]error),
		cancelErr:   make(map[string]error),
	}
}

func (m *mockManager) Schedule(_ context.Context, job *schedule.Job) error {
	if err, ok := m.scheduleErr[job.ID]; ok {
		return err
	}
	cp := *job
	m.jobs[job.ID] = &cp
	m.nextRun[job.ID] = time.Now().Add(time.Hour)
	return nil
}

func (m *mockManager) Cancel(_ context.Context, jobID string) error {
	if err, ok := m.cancelErr[jobID]; ok {
		return err
	}
	if _, ok := m.jobs[jobID]; !ok {
		return errors.New("not found")
	}
	delete(m.jobs, jobID)
	delete(m.nextRun, jobID)
	return nil
}

func (m *mockManager) NextRun(jobID string) (string, error) {
	if t, ok := m.nextRun[jobID]; ok {
		return t.Format(time.RFC3339), nil
	}
	return "unknown", errors.New("not found")
}

func (m *mockManager) List() []*schedule.Job {
	out := make([]*schedule.Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		cp := *j
		out = append(out, &cp)
	}
	return out
}

func assertResponse(t *testing.T, resp *tool.Response, err error, wantErr bool, substr string) {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		return
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, substr) {
		t.Fatalf("expected response containing %q, got %q", substr, text)
	}
}

func TestRegisterTools(t *testing.T) {
	mgr := newMockManager()
	tools := RegisterTools(mgr)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	names := []string{"ScheduleCreate", "ScheduleList", "ScheduleStop", "ScheduleView"}
	for i, want := range names {
		if tools[i].Name() != want {
			t.Fatalf("tool[%d]: expected %q, got %q", i, want, tools[i].Name())
		}
	}
}

func TestScheduleCreate_Success(t *testing.T) {
	mgr := newMockManager()
	create := NewCreateTool(mgr)
	ctx := context.Background()

	resp, err := create.Execute(ctx, map[string]any{
		"id":        "test-1",
		"agent_id":  "agent-1",
		"cron_expr": "@every 1h",
		"payload":   "run agent",
	})
	assertResponse(t, resp, err, false, "created")

	if _, ok := mgr.jobs["test-1"]; !ok {
		t.Fatal("job was not stored")
	}
	if mgr.jobs["test-1"].CronExpr != "@every 1h" {
		t.Fatalf("expected cron @every 1h, got %q", mgr.jobs["test-1"].CronExpr)
	}
	if mgr.jobs["test-1"].Payload != "run agent" {
		t.Fatalf("expected payload 'run agent', got %q", mgr.jobs["test-1"].Payload)
	}
}

func TestScheduleCreate_MissingRequired(t *testing.T) {
	mgr := newMockManager()
	create := NewCreateTool(mgr)

	tests := []struct {
		name  string
		input map[string]any
	}{
		{"empty", map[string]any{}},
		{"missing agent_id", map[string]any{"id": "t1", "cron_expr": "@daily", "payload": "x"}},
		{"missing cron_expr", map[string]any{"id": "t1", "agent_id": "a1", "payload": "x"}},
		{"missing id", map[string]any{"agent_id": "a1", "cron_expr": "@daily", "payload": "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := create.Execute(context.Background(), tt.input)
			assertResponse(t, resp, err, false, "ScheduleCreateError")
		})
	}
}

func TestScheduleCreate_ManagerError(t *testing.T) {
	mgr := newMockManager()
	mgr.scheduleErr["fail"] = errors.New("cron rejected")

	create := NewCreateTool(mgr)
	resp, err := create.Execute(context.Background(), map[string]any{
		"id": "fail", "agent_id": "a1", "cron_expr": "@daily", "payload": "x",
	})
	assertResponse(t, resp, err, false, "ScheduleCreateError: cron rejected")
}

func TestScheduleList_Empty(t *testing.T) {
	mgr := newMockManager()
	list := NewListTool(mgr)

	resp, err := list.Execute(context.Background(), nil)
	assertResponse(t, resp, err, false, "No schedules")
}

func TestScheduleList_NonEmpty(t *testing.T) {
	mgr := newMockManager()
	mgr.jobs["j1"] = &schedule.Job{ID: "j1", AgentID: "a1", CronExpr: "@daily"}
	mgr.jobs["j2"] = &schedule.Job{ID: "j2", AgentID: "a2", CronExpr: "@hourly"}
	mgr.nextRun["j1"] = time.Now().Add(time.Hour)
	mgr.nextRun["j2"] = time.Now().Add(2 * time.Hour)

	list := NewListTool(mgr)
	resp, err := list.Execute(context.Background(), nil)
	assertResponse(t, resp, err, false, "2 schedule(s)")
	if !strings.Contains(resp.GetTextContent(), "j1") || !strings.Contains(resp.GetTextContent(), "j2") {
		t.Fatal("expected both jobs in list output")
	}
}

func TestScheduleStop_Success(t *testing.T) {
	mgr := newMockManager()
	mgr.jobs["j1"] = &schedule.Job{ID: "j1", CronExpr: "@daily"}

	stop := NewStopTool(mgr)
	resp, err := stop.Execute(context.Background(), map[string]any{"id": "j1"})
	assertResponse(t, resp, err, false, "stopped")

	if _, ok := mgr.jobs["j1"]; ok {
		t.Fatal("job should have been removed")
	}
}

func TestScheduleStop_NotFound(t *testing.T) {
	mgr := newMockManager()
	stop := NewStopTool(mgr)

	resp, err := stop.Execute(context.Background(), map[string]any{"id": "does-not-exist"})
	assertResponse(t, resp, err, false, "ScheduleStopError")
}

func TestScheduleStop_MissingID(t *testing.T) {
	mgr := newMockManager()
	stop := NewStopTool(mgr)

	resp, err := stop.Execute(context.Background(), map[string]any{})
	assertResponse(t, resp, err, false, "ScheduleStopError")
}

func TestScheduleView_Success(t *testing.T) {
	mgr := newMockManager()
	mgr.jobs["j1"] = &schedule.Job{
		ID: "j1", AgentID: "a1", SessionID: "s1",
		CronExpr: "0 0 * * *", Payload: "daily task",
	}
	mgr.nextRun["j1"] = time.Now().Add(time.Hour)

	view := NewViewTool(mgr)
	resp, err := view.Execute(context.Background(), map[string]any{"id": "j1"})
	assertResponse(t, resp, err, false, "Schedule j1")
	if !strings.Contains(resp.GetTextContent(), "cron=0 0 * * *") {
		t.Fatal("expected cron expression in view output")
	}
	if !strings.Contains(resp.GetTextContent(), `payload="daily task"`) {
		t.Fatal("expected payload in view output")
	}
}

func TestScheduleView_NotFound(t *testing.T) {
	mgr := newMockManager()
	view := NewViewTool(mgr)

	resp, err := view.Execute(context.Background(), map[string]any{"id": "nope"})
	assertResponse(t, resp, err, false, "not found")
}

func TestScheduleView_MissingID(t *testing.T) {
	mgr := newMockManager()
	view := NewViewTool(mgr)

	resp, err := view.Execute(context.Background(), map[string]any{})
	assertResponse(t, resp, err, false, "not found")
}

func TestToolSpec_AllHaveDescriptions(t *testing.T) {
	mgr := newMockManager()
	tools := RegisterTools(mgr)
	for _, tl := range tools {
		spec := tl.Spec()
		if spec.Name == "" {
			t.Errorf("tool %T has empty name", tl)
		}
		if spec.Description == "" {
			t.Errorf("tool %T has empty description", tl)
		}
	}
}

func TestStrHelper(t *testing.T) {
	m := map[string]any{"key": "value", "num": 42}
	if str(m, "key") != "value" {
		t.Fatal("str failed for string value")
	}
	if str(m, "missing") != "" {
		t.Fatal("str should return empty for missing key")
	}
	if str(m, "num") != "" {
		t.Fatal("str should return empty for non-string value")
	}
}

func TestJoinLines(t *testing.T) {
	result := joinLines([]string{"a", "b", "c"})
	expected := "a\nb\nc"
	if result != expected {
		t.Fatalf("joinLines: expected %q, got %q", expected, result)
	}

	if joinLines(nil) != "" {
		t.Fatal("joinLines nil should return empty")
	}
	if joinLines([]string{"only"}) != "only" {
		t.Fatal("joinLines single item should not have prefix newline")
	}
}

func TestScheduleCreate_WithSessionID(t *testing.T) {
	mgr := newMockManager()
	create := NewCreateTool(mgr)

	resp, err := create.Execute(context.Background(), map[string]any{
		"id": "t1", "agent_id": "a1", "session_id": "s1",
		"cron_expr": "@daily", "payload": "x",
	})
	assertResponse(t, resp, err, false, "created")
	if mgr.jobs["t1"].SessionID != "s1" {
		t.Fatalf("expected session_id 's1', got %q", mgr.jobs["t1"].SessionID)
	}
}

func TestManagerInterfaceSatisfied(t *testing.T) {
	// compile-time check that mockManager satisfies Manager
	var _ Manager = (*mockManager)(nil)
}
