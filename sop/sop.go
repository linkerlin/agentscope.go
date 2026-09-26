// Package sop provides Standard Operating Procedures: fixed milestone
// sequences where each step is executed by an agent and judged before moving
// on. Definition carries no runtime state, so one SOP can drive any number of
// runs. Aligned with Python agentscope.sop (PyV2 #2393).
package sop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/output"
)

// errParked signals that the current step suspended on a human-in-the-loop
// event; the caller resumes with InjectEvent + another ReplyStream call.
var errParked = errors.New("sop: step parked awaiting external input")

// errInterruptedStep signals a user interruption; the attempt is discarded
// without consuming the max_attempts budget.
var errInterruptedStep = errors.New("sop: step interrupted")

// Phase is shared by steps and whole runs.
type Phase string

const (
	// PhasePending means the step has not started (or was sent back for rework).
	PhasePending Phase = "pending"
	// PhaseRunning means an attempt is executing.
	PhaseRunning Phase = "running"
	// PhaseAwaiting means the step suspended on external input.
	PhaseAwaiting Phase = "awaiting"
	// PhaseCompleted means the step passed verification.
	PhaseCompleted Phase = "completed"
	// PhaseFailed means the step exhausted attempts or errored.
	PhaseFailed Phase = "failed"
)

// VerificationResult records one settled judgment. Only settled verdicts are
// stored; the message is fed back to the executor verbatim.
type VerificationResult struct {
	Passed    bool      `json:"passed"`
	Message   string    `json:"message"`
	Verifier  string    `json:"verifier,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// StepState is the serializable runtime state of one step.
type StepState struct {
	Phase         Phase                `json:"phase"`
	Given         string               `json:"given"`
	Submission    string               `json:"submission,omitempty"`
	HasSubmission bool                 `json:"has_submission"`
	Verifications []VerificationResult `json:"verifications,omitempty"`
}

// RunState is the serializable runtime state of one SOP run.
type RunState struct {
	ID        string      `json:"id"`
	Inputs    string      `json:"inputs"`
	Steps     []StepState `json:"steps"`
	CreatedAt time.Time   `json:"created_at"`
}

// NewRunState creates an empty state sized for the given SOP.
func NewRunState(sop *SOP, inputs string) *RunState {
	states := make([]StepState, len(sop.Steps))
	for i := range states {
		states[i].Phase = PhasePending
	}
	return &RunState{
		ID:        uuid.New().String(),
		Inputs:    inputs,
		Steps:     states,
		CreatedAt: time.Now(),
	}
}

// Phase computes the run phase: all pending -> pending; any failed -> failed;
// all completed -> completed; any awaiting -> awaiting; otherwise running.
func (s *RunState) Phase() Phase {
	if s == nil || len(s.Steps) == 0 {
		return PhasePending
	}
	var sawPending, sawRunning, sawAwaiting, sawCompleted bool
	for _, st := range s.Steps {
		switch st.Phase {
		case PhaseFailed:
			return PhaseFailed
		case PhasePending:
			sawPending = true
		case PhaseAwaiting:
			sawAwaiting = true
		case PhaseCompleted:
			sawCompleted = true
		default:
			sawRunning = true
		}
	}
	switch {
	case sawAwaiting:
		return PhaseAwaiting
	case sawRunning:
		return PhaseRunning
	case sawCompleted && !sawPending:
		return PhaseCompleted
	case sawPending && !sawCompleted:
		return PhasePending
	default:
		return PhaseRunning
	}
}

// AgentLike is the agent surface SOP steps drive: event-stream execution with
// injection for suspend-resume, plus single-shot structured output for
// handover/verdict extraction.
type AgentLike interface {
	ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error)
	InjectEvent(ctx context.Context, ev event.AgentEvent) error
	CallStructured(ctx context.Context, user *message.Msg, schema *output.JSONSchema, target any) error
}

// Step is one milestone: an executor does the work, an optional verifier
// judges it. A nil verifier passes unconditionally.
type Step struct {
	// Subject is a short actionable name.
	Subject string
	// Description states the destination, not the route.
	Description string
	// MaxAttempts bounds tries (default 3).
	MaxAttempts int
	// Executor performs the step.
	Executor AgentLike
	// Verifier judges the submission; nil means unconditional pass.
	Verifier AgentLike
	// VerifierName labels verification records.
	VerifierName string
}

func (s Step) maxAttempts() int {
	if s.MaxAttempts <= 0 {
		return 3
	}
	return s.MaxAttempts
}

// SOP is a fixed milestone sequence.
type SOP struct {
	Name        string
	Description string
	Steps       []Step
}

// StepEvent is emitted on step start/end so UIs can track milestones.
type StepEvent struct {
	replyID string
	at      time.Time
	// Name is "sop_step_started" or "sop_step_ended".
	Name    string `json:"name"`
	SOP     string `json:"sop"`
	Step    int    `json:"step"`
	Subject string `json:"subject"`
	Attempt int    `json:"attempt"`
	Phase   Phase  `json:"phase"`
}

var _ event.AgentEvent = (*StepEvent)(nil)

// EventType implements event.AgentEvent.
func (e *StepEvent) EventType() string { return e.Name }

// Timestamp implements event.AgentEvent.
func (e *StepEvent) Timestamp() time.Time { return e.at }

// ReplyID implements event.AgentEvent.
func (e *StepEvent) ReplyID() string { return e.replyID }

// handoverResult is the executor's structured submission.
type handoverResult struct {
	Handover string `json:"handover"`
}

// verdictResult is the verifier's structured judgment.
type verdictResult struct {
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

func handoverSchema() *output.JSONSchema {
	return &output.JSONSchema{
		Type: "object",
		Properties: map[string]*output.SchemaProp{
			"handover": {
				Type:        "string",
				Description: "Concise handover of what was accomplished for the next step: facts, paths, decisions.",
			},
		},
		Required: []string{"handover"},
	}
}

func verdictSchema() *output.JSONSchema {
	return &output.JSONSchema{
		Type: "object",
		Properties: map[string]*output.SchemaProp{
			"passed":  {Type: "boolean"},
			"message": {Type: "string", Description: "For rejection: exactly what is missing; fed back to the executor verbatim."},
		},
		Required: []string{"passed", "message"},
	}
}

// execSession tracks a suspended executor stream for resume.
type execSession struct {
	agent     AgentLike
	ch        <-chan event.AgentEvent
	collected *message.Msg
	stepIdx   int
}

// Engine drives one SOP run. It resembles an agent (ReplyStream by name) so
// it can sit wherever agents go.
type Engine struct {
	sop       *SOP
	state     *RunState
	suspended *execSession
	replyID   string
}

// NewEngine creates an engine. A nil state starts a fresh run; a non-nil
// state must have exactly as many step states as the SOP has steps,
// otherwise the procedure changed and resuming would be unsound.
func NewEngine(sop *SOP, state *RunState) (*Engine, error) {
	if sop == nil {
		return nil, fmt.Errorf("sop: nil SOP")
	}
	if len(sop.Steps) == 0 {
		return nil, fmt.Errorf("sop %q: no steps", sop.Name)
	}
	for i := range sop.Steps {
		if strings.TrimSpace(sop.Steps[i].Subject) == "" {
			return nil, fmt.Errorf("sop %q: step %d has no subject", sop.Name, i+1)
		}
		if sop.Steps[i].Executor == nil {
			return nil, fmt.Errorf("sop %q: step %q has no executor", sop.Name, sop.Steps[i].Subject)
		}
	}
	if state != nil && len(state.Steps) != len(sop.Steps) {
		return nil, fmt.Errorf("sop %q: state has %d steps but the SOP has %d (procedure changed)",
			sop.Name, len(state.Steps), len(sop.Steps))
	}
	return &Engine{sop: sop, state: state}, nil
}

// State returns the current run state (serializable for persistence).
func (e *Engine) State() *RunState { return e.state }

// Suspended reports whether the engine is parked on external input.
func (e *Engine) Suspended() bool { return e != nil && e.suspended != nil }

// ReplyStream drives the run, forwarding the executor's events. A suspended
// step closes the stream (park); the caller injects the answer with
// InjectEvent and calls ReplyStream again to resume.
func (e *Engine) ReplyStream(ctx context.Context, inputs *message.Msg) (<-chan event.AgentEvent, error) {
	out := make(chan event.AgentEvent, 64)
	go func() {
		defer close(out)
		e.replyID = uuid.New().String()
		out <- &StepEvent{replyID: e.replyID, at: time.Now(), Name: "sop_run_started", SOP: e.sop.Name}
		e.drive(ctx, inputs, out)
		out <- &StepEvent{replyID: e.replyID, at: time.Now(), Name: "sop_run_ended", SOP: e.sop.Name, Phase: e.state.Phase()}
	}()
	return out, nil
}

// InjectEvent forwards an answer to the suspended executor.
func (e *Engine) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	if e.suspended == nil || e.suspended.agent == nil {
		return fmt.Errorf("sop: nothing suspended")
	}
	return e.suspended.agent.InjectEvent(ctx, ev)
}

func (e *Engine) drive(ctx context.Context, inputs *message.Msg, out chan<- event.AgentEvent) {
	startIdx := 0
	if e.suspended != nil {
		// Resuming: continue from the suspended step without rewriting
		// inputs or givens.
		startIdx = e.suspended.stepIdx
	} else {
		if e.state == nil {
			text := ""
			if inputs != nil {
				text = inputs.GetTextContent()
			}
			e.state = NewRunState(e.sop, text)
		}
	}
	for i := startIdx; i < len(e.sop.Steps); i++ {
		st := &e.state.Steps[i]
		if st.Phase == PhaseCompleted {
			continue
		}
		if st.Phase == PhaseFailed {
			return
		}
		e.runStep(ctx, i, out)
		if e.suspended != nil {
			return
		}
		if st.Phase == PhaseFailed {
			return
		}
	}
}

// handover builds a step's input: run inputs for step 0, otherwise the
// previous step's submission wrapped as a ledger (text only, no files or
// dialog history cross steps).
func (e *Engine) handover(index int) string {
	if index == 0 {
		return e.state.Inputs
	}
	prev := e.state.Steps[index-1]
	if !prev.HasSubmission || prev.Submission == "" {
		return ""
	}
	return fmt.Sprintf("<handover from=%q>\n%s\n</handover>",
		e.sop.Steps[index-1].Subject, prev.Submission)
}

func (e *Engine) runStep(ctx context.Context, index int, out chan<- event.AgentEvent) {
	step := e.sop.Steps[index]
	st := &e.state.Steps[index]
	// Assign this step's input from the previous step's submission. Skipped
	// when resuming the same step so the original given is preserved.
	if e.suspended == nil || e.suspended.stepIdx != index {
		st.Given = e.handover(index)
	}
	for len(st.Verifications) < step.maxAttempts() {
		st.Phase = PhaseRunning
		out <- &StepEvent{
			replyID: e.replyID, at: time.Now(), Name: "sop_step_started",
			SOP: e.sop.Name, Step: index, Subject: step.Subject,
			Attempt: len(st.Verifications) + 1, Phase: PhaseRunning,
		}
		handover, err := e.runExecutor(ctx, index, out)
		if err != nil {
			if errors.Is(err, errParked) {
				st.Phase = PhaseAwaiting
				return
			}
			if errors.Is(err, errInterruptedStep) {
				// The attempt is discarded without consuming the budget.
				st.Phase = PhasePending
				return
			}
			st.Phase = PhaseFailed
			return
		}
		st.Submission = handover
		st.HasSubmission = true
		if step.Verifier == nil {
			st.Verifications = append(st.Verifications, VerificationResult{Passed: true, CreatedAt: time.Now()})
			st.Phase = PhaseCompleted
			out <- &StepEvent{
				replyID: e.replyID, at: time.Now(), Name: "sop_step_ended",
				SOP: e.sop.Name, Step: index, Subject: step.Subject,
				Attempt: len(st.Verifications), Phase: PhaseCompleted,
			}
			return
		}
		verdict, err := e.runVerifier(ctx, step, st, handover)
		if err != nil {
			st.Phase = PhaseFailed
			return
		}
		st.Verifications = append(st.Verifications, *verdict)
		if verdict.Passed {
			st.Phase = PhaseCompleted
			out <- &StepEvent{
				replyID: e.replyID, at: time.Now(), Name: "sop_step_ended",
				SOP: e.sop.Name, Step: index, Subject: step.Subject,
				Attempt: len(st.Verifications), Phase: PhaseCompleted,
			}
			return
		}
		// Rejected: clear the submission and send the step back to pending so
		// the next attempt restarts from doing, not from judging.
		st.Submission = ""
		st.HasSubmission = false
		st.Phase = PhasePending
	}
	st.Phase = PhaseFailed
}

// runExecutor drives (or resumes) the executor's event stream, forwarding
// events. On park it suspends and returns errParked.
func (e *Engine) runExecutor(ctx context.Context, index int, out chan<- event.AgentEvent) (string, error) {
	step := e.sop.Steps[index]
	st := &e.state.Steps[index]
	var sess *execSession
	if e.suspended != nil && e.suspended.stepIdx == index {
		sess = e.suspended
		e.suspended = nil
	} else {
		instruction := fmt.Sprintf("Subject: %s\n\n%s\n\n<input>\n%s\n</input>\n\n"+
			"Complete the subject above. The input carries everything handed over so far.",
			step.Subject, step.Description, st.Given)
		ch, err := step.Executor.ReplyStream(ctx, userMsg(instruction))
		if err != nil {
			return "", err
		}
		sess = &execSession{
			agent:     step.Executor,
			ch:        ch,
			collected: message.NewMsg().Role(message.RoleAssistant).Build(),
			stepIdx:   index,
		}
	}
	for ev := range sess.ch {
		out <- ev
		sess.collected.AppendEvent(ev)
		if isParkEvent(ev) {
			e.suspended = sess
			return "", errParked
		}
		if _, ok := ev.(*event.InterruptEvent); ok {
			return "", errInterruptedStep
		}
		if _, ok := ev.(*event.ErrorEvent); ok {
			// Let the stream finish; a terminal error without content below
			// becomes a failed attempt via the handover extraction.
		}
	}
	var h handoverResult
	extract := fmt.Sprintf("The step work above produced this final reply:\n%s\n\n"+
		"Summarize the concrete handover for the next step: facts, paths, decisions.",
		sess.collected.GetTextContent())
	if err := step.Executor.CallStructured(ctx, userMsg(extract), handoverSchema(), &h); err != nil {
		return "", err
	}
	if strings.TrimSpace(h.Handover) == "" {
		return "", fmt.Errorf("sop %q step %q: empty handover", e.sop.Name, step.Subject)
	}
	return h.Handover, nil
}

// runVerifier judges one submission with a single structured call.
func (e *Engine) runVerifier(ctx context.Context, step Step, st *StepState, submission string) (*VerificationResult, error) {
	instruction := fmt.Sprintf(
		"<subject>\n%s\n\n%s\n</subject>\n\n<submission>\n%s\n</submission>\n\n"+
			"Judge whether the submission achieves the subject. "+
			"Return passed=true when it does; otherwise passed=false with message stating exactly what is missing.",
		step.Subject, step.Description, submission)
	var v verdictResult
	if err := step.Verifier.CallStructured(ctx, userMsg(instruction), verdictSchema(), &v); err != nil {
		return nil, err
	}
	name := step.VerifierName
	return &VerificationResult{
		Passed:    v.Passed,
		Message:   v.Message,
		Verifier:  name,
		CreatedAt: time.Now(),
	}, nil
}

// isParkEvent reports whether an event suspends the step for external input.
func isParkEvent(ev event.AgentEvent) bool {
	switch ev.(type) {
	case *event.RequireUserConfirmEvent, *event.RequireExternalExecutionEvent:
		return true
	}
	return false
}

func userMsg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
}
