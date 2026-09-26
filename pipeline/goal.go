package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/output"
)

// GoalRunner is the agent surface GoalPipeline drives. *react.ReActAgent
// satisfies it directly (Reply + CallStructured).
type GoalRunner interface {
	Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error)
	CallStructured(ctx context.Context, user *message.Msg, schema *output.JSONSchema, target any) error
}

// contextResetter lets GoalPipeline drop an agent's dialog context between
// attempts while keeping tool/task state. Implementations are optional.
type contextResetter interface {
	ResetDialogContext()
}

// GoalPipeline drives an executor agent toward a goal and lets a verifier
// judge each attempt until it passes, is declared impossible, or the
// iteration budget runs out. Aligned with Python GoalPipeline (PyV2 #2428).
type GoalPipeline struct {
	name string

	// Executor performs the work.
	Executor GoalRunner
	// Verifier judges the executor's report.
	Verifier GoalRunner

	// MaxIters bounds executor-verifier rounds (default 10).
	MaxIters int
	// VerifierResetContext clears the verifier's dialog context after each
	// failed attempt (tools/task state are kept).
	VerifierResetContext bool
}

// NewGoalPipeline creates a goal pipeline.
func NewGoalPipeline(name string, executor, verifier GoalRunner) *GoalPipeline {
	return &GoalPipeline{name: name, Executor: executor, Verifier: verifier}
}

// Name returns the pipeline name.
func (p *GoalPipeline) Name() string { return p.name }

// GoalResult is the outcome of Run.
type GoalResult struct {
	// Passed reports that the verifier accepted the final attempt.
	Passed bool
	// Impossible reports that the verifier declared the goal unreachable.
	Impossible bool
	// Report is the executor's final report (may be empty when it failed to
	// produce one in the last round).
	Report string
	// Verdict is the verifier's final result: pass, fail, or impossible.
	Verdict string
	// Iterations counts executor-verifier rounds actually run.
	Iterations int
}

// executionReport is the schema the executor must produce each round.
type executionReport struct {
	Report string `json:"report"`
}

// verification is the verifier's structured judgment.
type verification struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

const (
	verdictPass       = "pass"
	verdictFail       = "fail"
	verdictImpossible = "impossible"
)

func executionReportSchema() *output.JSONSchema {
	return &output.JSONSchema{
		Type: "object",
		Properties: map[string]*output.SchemaProp{
			"report": {
				Type: "string",
				Description: "What was accomplished: files/paths touched, how to run or verify it, " +
					"environment notes, and anything still blocking. Be concrete.",
			},
		},
		Required: []string{"report"},
	}
}

func verificationSchema() *output.JSONSchema {
	return &output.JSONSchema{
		Type: "object",
		Properties: map[string]*output.SchemaProp{
			"result": {
				Type:        "string",
				Enum:        []any{verdictPass, verdictFail, verdictImpossible},
				Description: "pass when the goal is fully achieved, fail when it needs another attempt, impossible when it cannot be achieved at all.",
			},
			"message": {
				Type:        "string",
				Description: "For fail: exactly what is missing or wrong; the executor receives this text verbatim.",
			},
		},
		Required: []string{"result", "message"},
	}
}

// Run executes the goal loop. The iteration budget is held on the pipeline,
// so callers resuming after human-in-the-loop interaction keep the same
// budget instead of resetting it (PyV2 parity).
func (p *GoalPipeline) Run(ctx context.Context, goal string) (*GoalResult, error) {
	if p == nil {
		return nil, fmt.Errorf("goal pipeline: nil pipeline")
	}
	if p.Executor == nil || p.Verifier == nil {
		return nil, fmt.Errorf("goal pipeline %s: executor and verifier are required", p.name)
	}
	if strings.TrimSpace(goal) == "" {
		return nil, fmt.Errorf("goal pipeline %s: goal cannot be empty", p.name)
	}
	maxIters := p.MaxIters
	if maxIters <= 0 {
		maxIters = 10
	}

	feedback := ""
	result := &GoalResult{}
	for iter := 0; iter < maxIters; iter++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		report, err := p.runExecutor(ctx, goal, feedback)
		if err != nil {
			return result, fmt.Errorf("goal pipeline %s: executor (iteration %d): %w", p.name, iter+1, err)
		}
		result.Report = report
		result.Iterations = iter + 1

		verdict, err := p.runVerifier(ctx, goal, report)
		if err != nil {
			return result, fmt.Errorf("goal pipeline %s: verifier (iteration %d): %w", p.name, iter+1, err)
		}
		result.Verdict = verdict.Result
		switch verdict.Result {
		case verdictPass:
			result.Passed = true
			return result, nil
		case verdictImpossible:
			result.Impossible = true
			return result, nil
		default:
			// Unknown verdicts are treated as fail so the loop keeps making
			// progress instead of silently accepting a malformed judgment.
			feedback = verdict.Message
		}
		if p.VerifierResetContext {
			if r, ok := p.Verifier.(contextResetter); ok {
				r.ResetDialogContext()
			}
		}
	}
	return result, nil
}

func (p *GoalPipeline) runExecutor(ctx context.Context, goal, feedback string) (string, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Goal: %s\n\n", goal)
	sb.WriteString("<system-reminder>Work toward the goal, then report what you accomplished: " +
		"files/paths touched, how to run or verify it, environment notes, and anything still blocking.</system-reminder>")
	if feedback != "" {
		fmt.Fprintf(&sb, "\n\nThe verifier rejected the previous attempt:\n<feedback>\n%s\n</feedback>\n"+
			"Address this feedback and produce an updated report.", feedback)
	}
	var report executionReport
	if err := p.Executor.CallStructured(ctx, userMsg(sb.String()), executionReportSchema(), &report); err != nil {
		return "", err
	}
	return report.Report, nil
}

func (p *GoalPipeline) runVerifier(ctx context.Context, goal, report string) (*verification, error) {
	instruction := fmt.Sprintf(
		"<goal>\n%s\n</goal>\n\n<report>\n%s\n</report>\n\n"+
			"Judge whether the report shows the goal is fully achieved. "+
			"Return result=pass when it is, result=fail with the missing parts in message when another attempt should be made, "+
			"and result=impossible when the goal cannot be achieved.",
		goal, report)
	var verdict verification
	if err := p.Verifier.CallStructured(ctx, userMsg(instruction), verificationSchema(), &verdict); err != nil {
		return nil, err
	}
	return &verdict, nil
}

func userMsg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
}
