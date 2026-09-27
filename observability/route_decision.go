package observability

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
)

// RouteDecision tracing (16.3): CostRouter attaches its decision to the
// request context; this helper lands it on a tracing span via the
// Tracer abstraction so the routing choice is queryable in OTel/Langfuse/
// LangSmith alongside the model-call spans.

// RouteDecisionSpanAttrs renders one RouteDecision as span attributes with
// the agentscope.route.* key convention.
func RouteDecisionSpanAttrs(dec *model.RouteDecision) []SpanAttr {
	if dec == nil {
		return nil
	}
	return []SpanAttr{
		StringAttr("agentscope.route.primary", dec.Primary),
		StringAttr("agentscope.route.chosen", dec.Chosen),
		BoolAttr("agentscope.route.downgraded", dec.Downgraded),
		BoolAttr("agentscope.route.escalated", dec.Escalated),
		Float64Attr("agentscope.route.estimated_cost", dec.EstimatedCost),
		StringAttr("agentscope.route.reason", dec.Reason),
	}
}

// TraceRouteDecision records one RouteDecision as a short-lived span on the
// given tracer. Use it from a CostRouter decision observer (model
// .WithRouteDecisionObserver) so every routing choice lands in tracing:
//
//	cr := model.NewCostRouter(primary,
//	    model.WithRouteDecisionObserver(func(dec model.RouteDecision) {
//	        observability.TraceRouteDecision(ctx, tracer, dec)
//	    }), ...)
func TraceRouteDecision(ctx context.Context, tracer Tracer, dec model.RouteDecision) {
	if tracer == nil {
		return
	}
	_, span := tracer.Start(ctx, "route_decision")
	span.SetAttributes(RouteDecisionSpanAttrs(&dec)...)
	span.End()
}
