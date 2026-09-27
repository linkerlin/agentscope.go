package model

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// RouteDecision records which model a CostRouter chose for one call and why.
// Observers (tracing) receive every decision; the decision also travels on
// the request context so downstream tracing middleware can read it.
type RouteDecision struct {
	Primary       string  `json:"primary"`
	Chosen        string  `json:"chosen"`
	Downgraded    bool    `json:"downgraded"`
	Escalated     bool    `json:"escalated"` // downgraded call failed -> flagship fallback
	EstimatedCost float64 `json:"estimated_cost"`
	Reason        string  `json:"reason"`
}

type routeDecisionCtxKey struct{}

// RouteDecisionFromContext returns the decision a CostRouter attached to the
// request context, if any.
func RouteDecisionFromContext(ctx context.Context) (*RouteDecision, bool) {
	dec, ok := ctx.Value(routeDecisionCtxKey{}).(*RouteDecision)
	return dec, ok
}

// CostRouter is an optional cost-aware extension of Router (16.3): a pure
// rule layer reads model prices and a per-turn budget and may downgrade a
// request to a cheaper configured model. Default-off: it only exists when the
// caller constructs one, and with no pricing/policy configured it routes
// exactly like Router (primary with retry + fallback on failure; the
// flagship is always reachable — a downgraded call that fails escalates back
// to the primary).
type CostRouter struct {
	primary    ChatModel
	fallback   ChatModel
	maxRetries int
	backoff    time.Duration
	cb         *circuitBreaker

	prices            map[string]ModelPricing
	maxCostPerTurn    float64
	outTokensEstimate int
	onDecision        func(RouteDecision)
}

// CostRouterOption configures a CostRouter.
type CostRouterOption func(*CostRouter)

// WithCostFallback sets the cheap model used when the policy downgrades.
func WithCostFallback(m ChatModel) CostRouterOption {
	return func(r *CostRouter) { r.fallback = m }
}

// WithCostMaxRetries sets the retry count per chosen model (default 1).
func WithCostMaxRetries(n int) CostRouterOption {
	return func(r *CostRouter) { r.maxRetries = n }
}

// WithCostBackoff sets the backoff between retries.
func WithCostBackoff(d time.Duration) CostRouterOption {
	return func(r *CostRouter) { r.backoff = d }
}

// WithModelPrice registers the price for one model name. Unknown models are
// treated as un-estimatable and never downgraded (conservative).
func WithModelPrice(modelName string, p ModelPricing) CostRouterOption {
	return func(r *CostRouter) { r.prices[modelName] = p }
}

// WithModelCardPrice registers pricing from a ModelCard carrying Pricing data.
// Cards without pricing are ignored.
func WithModelCardPrice(card *ModelCard) CostRouterOption {
	return func(r *CostRouter) {
		if card != nil && card.Pricing != nil && card.ID != "" {
			r.prices[card.ID] = *card.Pricing
		}
	}
}

// WithMaxCostPerTurn sets the per-turn budget in dollars. Zero (default)
// disables cost routing entirely — behavior identical to Router.
func WithMaxCostPerTurn(v float64) CostRouterOption {
	return func(r *CostRouter) { r.maxCostPerTurn = v }
}

// WithOutputTokenEstimate sets the output token assumption used for cost
// estimation (default 1000).
func WithOutputTokenEstimate(n int) CostRouterOption {
	return func(r *CostRouter) { r.outTokensEstimate = n }
}

// WithRouteDecisionObserver registers a callback receiving every
// RouteDecision — the tracing hook. Observability code (or the gateway) uses
// it to record the decision; the decision is also attached to the request
// context for downstream span attributes.
func WithRouteDecisionObserver(fn func(RouteDecision)) CostRouterOption {
	return func(r *CostRouter) { r.onDecision = fn }
}

// WithCostCircuitBreaker enables the three-state circuit breaker.
func WithCostCircuitBreaker(threshold int, cooldown time.Duration) CostRouterOption {
	return func(r *CostRouter) { r.cb = newCircuitBreaker(threshold, cooldown) }
}

// NewCostRouter creates a cost-aware router. With no pricing registered and
// no MaxCostPerTurn set, it behaves exactly like NewRouter.
func NewCostRouter(primary ChatModel, opts ...CostRouterOption) *CostRouter {
	r := &CostRouter{
		primary:           primary,
		maxRetries:        1,
		backoff:           500 * time.Millisecond,
		prices:            make(map[string]ModelPricing),
		outTokensEstimate: 1000,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *CostRouter) ModelName() string { return r.primary.ModelName() }

// Chat routes one non-streaming call through the cost policy.
func (r *CostRouter) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	dec := r.decide(messages)
	ctx = r.record(ctx, dec)
	chosen := r.primary
	if dec.Downgraded {
		chosen = r.fallback
	}
	resp, err := r.callRetry(ctx, chosen, messages, options)
	if err != nil && dec.Downgraded && r.primary != nil {
		// Flagship fallback: the downgraded model failing must not lose the
		// turn — escalate back to the primary.
		esc := RouteDecision{Primary: dec.Primary, Chosen: dec.Primary, Escalated: true, Reason: "downgraded call failed: " + err.Error()}
		ctx = r.record(ctx, esc)
		return r.callRetry(ctx, r.primary, messages, options)
	}
	return resp, err
}

// ChatStream routes one streaming call through the cost policy.
func (r *CostRouter) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	dec := r.decide(messages)
	ctx = r.record(ctx, dec)
	chosen := r.primary
	if dec.Downgraded {
		chosen = r.fallback
	}
	ch, err := r.streamRetry(ctx, chosen, messages, options)
	if err != nil && dec.Downgraded && r.primary != nil {
		esc := RouteDecision{Primary: dec.Primary, Chosen: dec.Primary, Escalated: true, Reason: "downgraded call failed: " + err.Error()}
		r.record(ctx, esc)
		return r.streamRetry(ctx, r.primary, messages, options)
	}
	return ch, err
}

// decide applies the rule layer. With no policy configured it always picks
// the primary (Router parity).
func (r *CostRouter) decide(messages []*message.Msg) RouteDecision {
	dec := RouteDecision{Primary: r.primary.ModelName(), Chosen: r.primary.ModelName()}
	if r.maxCostPerTurn <= 0 || r.fallback == nil {
		dec.Reason = "no cost policy configured"
		return dec
	}
	pc, ok := r.prices[dec.Primary]
	if !ok {
		dec.Reason = "primary unpriced"
		return dec
	}
	fc, ok := r.prices[r.fallback.ModelName()]
	if !ok || fc.total() >= pc.total() {
		dec.Reason = "no cheaper priced fallback"
		return dec
	}
	est := r.estimate(pc, messages)
	if est <= r.maxCostPerTurn {
		dec.EstimatedCost = est
		dec.Reason = "within budget"
		return dec
	}
	dec.Chosen = r.fallback.ModelName()
	dec.Downgraded = true
	dec.EstimatedCost = r.estimate(fc, messages)
	dec.Reason = fmt.Sprintf("primary estimate $%.4f exceeds per-turn budget $%.4f", est, r.maxCostPerTurn)
	return dec
}

func (p ModelPricing) total() float64 { return p.InputPerMTok + p.OutputPerMTok }

// estimatePrices approximates input tokens at 4 chars/token (utf8 heuristic,
// matching rag.ApproxTokenChunker) plus the configured output estimate.
func (r *CostRouter) estimate(p ModelPricing, messages []*message.Msg) float64 {
	var chars int
	for _, m := range messages {
		chars += len(m.GetTextContent())
	}
	in := float64(chars) / 4.0
	out := float64(r.outTokensEstimate)
	return (in*p.InputPerMTok + out*p.OutputPerMTok) / 1_000_000.0
}

func (r *CostRouter) record(ctx context.Context, dec RouteDecision) context.Context {
	if r.onDecision != nil {
		r.onDecision(dec)
	}
	return context.WithValue(ctx, routeDecisionCtxKey{}, &dec)
}

func (r *CostRouter) callRetry(ctx context.Context, m ChatModel, messages []*message.Msg, options []ChatOption) (*message.Msg, error) {
	if r.cb != nil {
		if err := r.cb.allowRequest(); err != nil {
			return nil, err
		}
	}
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 && r.backoff > 0 {
			select {
			case <-time.After(r.backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, err := m.Chat(ctx, messages, options...)
		if err == nil {
			if r.cb != nil {
				r.cb.onSuccess()
			}
			return resp, nil
		}
		lastErr = err
	}
	if r.cb != nil {
		r.cb.onFailure()
	}
	return nil, fmt.Errorf("cost router: %s failed after %d retries: %w", m.ModelName(), r.maxRetries, lastErr)
}

func (r *CostRouter) streamRetry(ctx context.Context, m ChatModel, messages []*message.Msg, options []ChatOption) (<-chan *StreamChunk, error) {
	if r.cb != nil {
		if err := r.cb.allowRequest(); err != nil {
			return nil, err
		}
	}
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 && r.backoff > 0 {
			select {
			case <-time.After(r.backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		ch, err := m.ChatStream(ctx, messages, options...)
		if err == nil {
			if r.cb != nil {
				r.cb.onSuccess()
			}
			return ch, nil
		}
		lastErr = err
	}
	if r.cb != nil {
		r.cb.onFailure()
	}
	return nil, fmt.Errorf("cost router: %s stream failed after %d retries: %w", m.ModelName(), r.maxRetries, lastErr)
}

// Ensure CostRouter implements ChatModel at compile time.
var _ ChatModel = (*CostRouter)(nil)
