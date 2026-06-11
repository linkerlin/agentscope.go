package gateway

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/schedule"
	"github.com/linkerlin/agentscope.go/tool"
)

type fakeScheduleMgr struct{}

func (f fakeScheduleMgr) Schedule(ctx context.Context, job *schedule.Job) error {
	return nil
}
func (f fakeScheduleMgr) Cancel(ctx context.Context, id string) error { return nil }
func (f fakeScheduleMgr) NextRun(id string) (string, error)           { return "", nil }
func (f fakeScheduleMgr) List() []*schedule.Job                       { return nil }

func TestStandardTools_Basic(t *testing.T) {
	tools := StandardTools(StandardToolsOptions{
		WorkspaceDir: "/tmp/testws",
		ReadOnly:     true,
	})

	if len(tools) < 3 {
		t.Fatalf("expected at least some file tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, t := range tools {
		names[t.Name()] = true
	}
	// RegisterAll (readOnly) gives: read_file (or similar), glob, grep
	if !names["read_file"] && !names["glob"] && !names["grep"] {
		t.Errorf("expected core file tools from RegisterAll, got names: %v", names)
	}
}

func TestStandardTools_WithTaskAndSchedule(t *testing.T) {
	tools := StandardTools(StandardToolsOptions{
		WorkspaceDir:    ".",
		IncludeTask:     true,
		TaskStore:       nil, // real callers should pass a scheduletask.Store
		IncludeSchedule: true,
		ScheduleMgr:     fakeScheduleMgr{},
	})

	hasSched := false
	for _, t := range tools {
		if t.Name() == "ScheduleCreate" || t.Name() == "ScheduleList" {
			hasSched = true
		}
	}
	if !hasSched {
		t.Error("expected schedule tools when ScheduleMgr provided")
	}
}

func TestStandardTools_Extra(t *testing.T) {
	extra := tool.NewFunctionTool("hello", "say hi", nil,
		func(ctx context.Context, in map[string]any) (*tool.Response, error) {
			return tool.NewTextResponse("hi"), nil
		})
	tools := StandardTools(StandardToolsOptions{Extra: []tool.Tool{extra}})
	found := false
	for _, t := range tools {
		if t.Name() == "hello" {
			found = true
		}
	}
	if !found {
		t.Error("extra tool not included")
	}
}
