// Package middleware control-plane integration: an on_reply interceptor that
// asks the control-plane Kernel whether this turn should run, and short-circuits
// the reply with a structured "blocked" assistant message when it should not
// (unresolved gate, paused goal, or exhausted quota).
package middleware

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/controlplane"
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
// Writeback and spend are NOT done here: the control plane does not execute
// work. After a turn that ShouldRun allowed, the runtime (or a thin wrapper)
// calls Kernel.Writeback + Kernel.SpendSlot separately, often via the gateway
// /api/v1/controlplane/* routes.
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
		return next(ctx)
	}
	return blockedReply(dec)
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
