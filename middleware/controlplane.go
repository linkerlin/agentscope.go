// Package middleware control-plane integration: an on_reply interceptor that
// asks the control-plane Kernel whether this turn should run, and short-circuits
// the reply with a structured "blocked" assistant message when it should not
// (unresolved gate, paused goal, or exhausted quota).
package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/logging"
	"github.com/linkerlin/agentscope.go/message"
)

// ControlPlaneMiddleware gates each agent reply through the control plane's
// ShouldRun decision. It is the runtime hook that makes the LoopX-style
// governance layer actually govern turns: when ShouldRun returns false, the
// reply is short-circuited with a structured assistant message instead of
// running the model loop.
//
// Binding policy is injected via GoalResolver: the middleware does not assume
// how sessions map to lifetime goals. If a goal is not bound for the acting
// agent (ok=false), the turn passes through unchanged — so attaching this
// middleware with no bindings is a no-op (default-off, faithful to the plan).
//
// Governance hot path (16.4): after a turn that ShouldRun allowed and that
// completed without error, the middleware automatically records one
// Writeback (progress outcome plus a turn-reply evidence item) and one
// SpendSlot for the goal's current todo. A failed turn never writes back and
// never spends; a Writeback failure never spends ("success spends once").
// Accounting failures are logged and swallowed — a control-plane hiccup must
// not break the agent, matching the passthrough philosophy of ShouldRun.
// Turns with no current todo on the goal have nothing accountable to record
// and are skipped.
type ControlPlaneMiddleware struct {
	Base
	// Kernel is the governance kernel. nil = middleware is a no-op.
	Kernel *controlplane.Kernel
	// GoalResolver maps the acting agent id to the lifetime goal it currently
	// works on. Return ok=false to passthrough (no goal bound).
	GoalResolver func(agentID string) (goalID string, ok bool)
	// AgentIDFromName optionally maps the middleware Agent.AgentName() to the
	// registered control-plane agent id. nil = use AgentName() as-is.
	AgentIDFromName func(name string) string
}

// OnReply implements ReplyInterceptor. It consults ShouldRun before the reply;
// if the turn is blocked it returns a descriptive assistant message instead of
// calling next. ShouldRun errors are non-fatal (passthrough) so a control-plane
// hiccup never breaks the agent.
func (m *ControlPlaneMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	if m.Kernel == nil || m.GoalResolver == nil {
		return next(ctx)
	}
	agentID := agent.AgentName()
	if m.AgentIDFromName != nil {
		agentID = m.AgentIDFromName(agentID)
	}
	goalID, ok := m.GoalResolver(agentID)
	if !ok || goalID == "" {
		return next(ctx) // no goal bound -> passthrough
	}
	dec, err := m.Kernel.ShouldRun(ctx, goalID, agentID)
	if err != nil || dec == nil || dec.ShouldRun {
		resp, replyErr := next(ctx)
		if replyErr == nil && resp != nil {
			m.accountTurn(ctx, goalID, agentID, resp)
		}
		return resp, replyErr
	}
	return blockedReply(dec)
}

// accountTurn records one Writeback + one SpendSlot for a successfully
// completed turn (16.4 governance hot path). Best-effort by design: any
// failure is logged and swallowed so accounting can never break the reply.
func (m *ControlPlaneMiddleware) accountTurn(ctx context.Context, goalID, agentID string, resp *message.Msg) {
	defer func() {
		if r := recover(); r != nil {
			logging.Default().Error("controlplane middleware: account turn panicked", "panic", r)
		}
	}()

	goal, err := m.Kernel.GoalStore().Get(ctx, goalID)
	if err != nil || goal == nil || goal.CurrentTodoID == "" {
		// Nothing accountable to record for this turn.
		return
	}

	turnID := uuid.NewString()
	_, err = m.Kernel.Writeback(ctx, controlplane.ValidatedWriteback{
		TodoID:  goal.CurrentTodoID,
		GoalID:  goalID,
		TurnID:  turnID,
		AgentID: agentID,
		// Progress (not completion): finishing a turn is real, evidenced
		// work, but completion semantics (todo done + lane gates) belong to
		// explicit evaluation, not to every successful chat turn.
		Outcome:        controlplane.Outcome{Status: controlplane.OutcomeProgress},
		DecisionSource: "runtime_auto",
		Evidence: []controlplane.Evidence{{
			ID:         uuid.NewString(),
			Kind:       "turn_reply",
			Summary:    truncateSummary(resp.GetTextContent()),
			ProducedAt: time.Now(),
		}},
	})
	if err != nil {
		logging.Default().Warn("controlplane middleware: writeback failed (no spend)", "error", err)
		return
	}
	if _, err := m.Kernel.SpendSlot(ctx, goalID, turnID, controlplane.SpendOpts{
		Execute: true,
		Reason:  "auto: turn delivered",
	}); err != nil {
		logging.Default().Warn("controlplane middleware: spend failed after writeback", "error", err)
	}
}

// truncateSummary keeps the evidence summary compact and single-line.
func truncateSummary(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) > 200 {
		s = s[:197] + "..."
	}
	return s
}

// blockedReply renders a ShouldRun=false decision as an assistant message the
// user/operator can act on. The wording names the concrete gate question when
// one is open (never a vague "waiting"), or the pause/quota reason otherwise.
func blockedReply(dec *controlplane.Decision) (*message.Msg, error) {
	text := blockText(dec)
	return message.NewMsg().Role(message.RoleAssistant).TextContent(text).Build(), nil
}

func blockText(dec *controlplane.Decision) string {
	switch dec.State {
	case controlplane.ComputeOperatorGate:
		if dec.Question != "" {
			if dec.FallbackAuthorized {
				return fmt.Sprintf(
					"[control-plane] Blocked by user gate (%s): %s\nA scoped fallback is authorized: %s",
					dec.GateID, dec.Question, dec.Fallback.Action)
			}
			return fmt.Sprintf(
				"[control-plane] Blocked by user gate (%s): %s\nAwaiting operator decision before this lane may proceed.",
				dec.GateID, dec.Question)
		}
		return fmt.Sprintf("[control-plane] Blocked by an unresolved user gate (%s).", dec.GateID)
	case controlplane.ComputePaused:
		return "[control-plane] Goal is paused; turn skipped (all automatic permissions false)."
	case controlplane.ComputeThrottled:
		return fmt.Sprintf(
			"[control-plane] Quota exhausted in the rolling window (spent %d / allowed %d); backing off.",
			dec.Spent, dec.Allowed)
	default:
		if dec.Reason != "" {
			return fmt.Sprintf("[control-plane] Turn not run: %s.", dec.Reason)
		}
		return "[control-plane] Turn not run."
	}
}
