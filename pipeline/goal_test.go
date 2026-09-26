package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/output"
)

// scriptedRunner serves scripted reports (executor) or verdicts (verifier)
// and records the instructions it received.
type scriptedRunner struct {
	reports      []string
	verdicts     []verification
	instructions []string
	resetCount   int
}

func (r *scriptedRunner) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (r *scriptedRunner) CallStructured(ctx context.Context, user *message.Msg, schema *output.JSONSchema, target any) error {
	r.instructions = append(r.instructions, user.GetTextContent())
	switch v := target.(type) {
	case *executionReport:
		if len(r.reports) == 0 {
			return errors.New("no report scripted")
		}
		v.Report = r.reports[0]
		r.reports = r.reports[1:]
	case *verification:
		if len(r.verdicts) == 0 {
			return errors.New("no verdict scripted")
		}
		*v = r.verdicts[0]
		r.verdicts = r.verdicts[1:]
	default:
		return errors.New("unexpected target type")
	}
	return nil
}

func (r *scriptedRunner) ResetDialogContext() { r.resetCount++ }

func TestGoalPipeline_PassFirstTry(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"done: created main.go"}}
	ver := &scriptedRunner{verdicts: []verification{{Result: "pass", Message: "looks good"}}}
	p := NewGoalPipeline("g", exec, ver)

	res, err := p.Run(context.Background(), "create a hello world program")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed || res.Impossible || res.Iterations != 1 || res.Report != "done: created main.go" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !strings.Contains(exec.instructions[0], "create a hello world program") {
		t.Fatalf("goal missing from executor instruction: %q", exec.instructions[0])
	}
	if !strings.Contains(ver.instructions[0], "<report>") {
		t.Fatalf("report missing from verifier instruction: %q", ver.instructions[0])
	}
}

func TestGoalPipeline_FailThenPass(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"attempt 1", "attempt 2"}}
	ver := &scriptedRunner{verdicts: []verification{
		{Result: "fail", Message: "tests are missing"},
		{Result: "pass", Message: ""},
	}}
	p := NewGoalPipeline("g", exec, ver)

	res, err := p.Run(context.Background(), "ship the feature")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed || res.Iterations != 2 || res.Report != "attempt 2" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !strings.Contains(exec.instructions[1], "tests are missing") {
		t.Fatalf("verifier feedback not fed back to executor: %q", exec.instructions[1])
	}
}

func TestGoalPipeline_Impossible(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"cannot access the network"}}
	ver := &scriptedRunner{verdicts: []verification{{Result: "impossible", Message: "no network"}}}
	p := NewGoalPipeline("g", exec, ver)

	res, err := p.Run(context.Background(), "download the dataset")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Impossible || res.Passed || res.Verdict != "impossible" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestGoalPipeline_MaxItersExhausted(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"a", "b", "c"}}
	ver := &scriptedRunner{verdicts: []verification{
		{Result: "fail", Message: "nope"},
		{Result: "fail", Message: "nope"},
		{Result: "fail", Message: "nope"},
	}}
	p := NewGoalPipeline("g", exec, ver)
	p.MaxIters = 3

	res, err := p.Run(context.Background(), "hard goal")
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed || res.Impossible || res.Iterations != 3 {
		t.Fatalf("expected exhausted budget, got %+v", res)
	}
}

func TestGoalPipeline_UnknownVerdictTreatedAsFail(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"a", "b"}}
	ver := &scriptedRunner{verdicts: []verification{
		{Result: "maybe", Message: "unclear"},
		{Result: "pass", Message: ""},
	}}
	p := NewGoalPipeline("g", exec, ver)

	res, err := p.Run(context.Background(), "g")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed || res.Iterations != 2 {
		t.Fatalf("unknown verdict should behave as fail, got %+v", res)
	}
}

func TestGoalPipeline_VerifierResetContext(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"a", "b"}}
	ver := &scriptedRunner{verdicts: []verification{
		{Result: "fail", Message: "again"},
		{Result: "pass", Message: ""},
	}}
	p := NewGoalPipeline("g", exec, ver)
	p.VerifierResetContext = true

	if _, err := p.Run(context.Background(), "g"); err != nil {
		t.Fatal(err)
	}
	if ver.resetCount != 1 {
		t.Fatalf("expected verifier context reset once, got %d", ver.resetCount)
	}
}

func TestGoalPipeline_Validation(t *testing.T) {
	exec := &scriptedRunner{}
	ver := &scriptedRunner{}
	if _, err := NewGoalPipeline("g", exec, nil).Run(context.Background(), "x"); err == nil {
		t.Fatal("expected error for nil verifier")
	}
	if _, err := NewGoalPipeline("g", nil, ver).Run(context.Background(), "x"); err == nil {
		t.Fatal("expected error for nil executor")
	}
	if _, err := NewGoalPipeline("g", exec, ver).Run(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty goal")
	}
	if _, err := (*GoalPipeline)(nil).Run(context.Background(), "x"); err == nil {
		t.Fatal("expected error for nil pipeline")
	}
}

func TestGoalPipeline_ContextCancelled(t *testing.T) {
	exec := &scriptedRunner{reports: []string{"a"}}
	ver := &scriptedRunner{verdicts: []verification{{Result: "fail", Message: "again"}}}
	p := NewGoalPipeline("g", exec, ver)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Run(ctx, "g"); err == nil {
		t.Fatal("expected context error")
	}
}
