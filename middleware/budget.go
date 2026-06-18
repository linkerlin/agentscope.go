package middleware

import (
	"context"
	"sync"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// DefaultBudgetHintMessage is injected into the agent's context when the token
// budget is exhausted, instructing the agent to wrap up without further tool
// calls. Mirrors Python agentscope's BudgetControlMiddleware (#1738).
const DefaultBudgetHintMessage = "<system-reminder>You have reached the maximum token budget set by the " +
	"user. Now you MUST wrap up immediately and provide a final " +
	"concluding response without invoking any tools." +
	"</system-reminder>"

// budgetState is the per-reply weighted-cost accumulator. It is carried via
// context.Context, which is shared across the on_reply / on_model_call /
// on_reasoning hooks within a single reply (the same ctx flows from
// Base.Call down into the reasoning and model-call chains).
type budgetState struct {
	mu   sync.Mutex
	used float64
}

type budgetCtxKey struct{}

// BudgetControlMiddleware enforces a weighted token budget per reply.
//
// It tracks cumulative weighted token usage across all reasoning steps within
// a single reply:
//
//	cost = InputTokenWeight * input_tokens + OutputTokenWeight * output_tokens
//
// Once the accumulated cost reaches TokenBudget, a hint message is injected
// before the next reasoning step and tool_choice is forced to "none" so the
// agent wraps up without invoking any further tools.
//
// Budget state lives in the per-reply context.Context, so the middleware
// instance itself is stateless and safe to share across multiple agents /
// concurrent replies.
//
// Coverage: this middleware intercepts the synchronous reply lifecycle
// (Reply -> reasoning -> model call onion chain). It accumulates usage from
// the model-call response and enforces the budget on the next reasoning step.
type BudgetControlMiddleware struct {
	Base
	// TokenBudget is the maximum weighted token cost allowed per reply.
	TokenBudget float64
	// InputTokenWeight multiplies input tokens when computing the cost.
	InputTokenWeight float64
	// OutputTokenWeight multiplies output tokens when computing the cost.
	// Set higher than InputTokenWeight to reflect that output tokens are
	// typically more expensive.
	OutputTokenWeight float64
	// HintMessage is injected into the context when the budget is exceeded.
	HintMessage string
}

// NewBudgetControlMiddleware creates a BudgetControlMiddleware with equal
// input/output token weights and the default hint message.
func NewBudgetControlMiddleware(tokenBudget float64) *BudgetControlMiddleware {
	return &BudgetControlMiddleware{
		TokenBudget:       tokenBudget,
		InputTokenWeight:  1,
		OutputTokenWeight: 1,
		HintMessage:       DefaultBudgetHintMessage,
	}
}

// WithInputTokenWeight sets the input token weight (builder-style).
func (m *BudgetControlMiddleware) WithInputTokenWeight(w float64) *BudgetControlMiddleware {
	m.InputTokenWeight = w
	return m
}

// WithOutputTokenWeight sets the output token weight (builder-style).
func (m *BudgetControlMiddleware) WithOutputTokenWeight(w float64) *BudgetControlMiddleware {
	m.OutputTokenWeight = w
	return m
}

// WithHintMessage overrides the default hint message (builder-style).
func (m *BudgetControlMiddleware) WithHintMessage(hint string) *BudgetControlMiddleware {
	m.HintMessage = hint
	return m
}

// OnReply initializes a fresh per-reply budget accumulator in the context so
// each reply starts from zero.
func (m *BudgetControlMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	return next(context.WithValue(ctx, budgetCtxKey{}, &budgetState{}))
}

// OnModelCall accumulates the weighted token usage from the model response.
// Usage is read from msg.Usage (populated on non-streaming calls) and falls
// back to msg.Metadata["usage"] (model.ChatUsage, populated on streaming
// calls).
func (m *BudgetControlMiddleware) OnModelCall(ctx context.Context, agent Agent, input *ModelCallInput, next ModelCallNext) (*message.Msg, error) {
	msg, err := next(ctx)
	if err != nil || msg == nil {
		return msg, err
	}
	st, _ := ctx.Value(budgetCtxKey{}).(*budgetState)
	if st == nil {
		// No reply-scoped state (e.g. model call outside a wrapped reply);
		// nothing to accumulate.
		return msg, err
	}
	inTok, outTok := tokenUsageFromMsg(msg)
	if inTok == 0 && outTok == 0 {
		return msg, err
	}
	cost := m.InputTokenWeight*float64(inTok) + m.OutputTokenWeight*float64(outTok)
	st.mu.Lock()
	st.used += cost
	st.mu.Unlock()
	return msg, err
}

// OnReasoning checks the accumulated budget before each reasoning step. If the
// budget is exhausted, it injects a hint message into the reasoning input's
// message history and forces tool_choice to "none" so the agent stops calling
// tools and wraps up.
func (m *BudgetControlMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	if st, ok := ctx.Value(budgetCtxKey{}).(*budgetState); ok && st != nil {
		st.mu.Lock()
		used := st.used
		st.mu.Unlock()
		if used >= m.TokenBudget {
			injectBudgetHint(input, agent.AgentName(), m.HintMessage)
			input.ChatOpts = append(input.ChatOpts, model.WithToolChoice(&model.ToolChoice{Mode: "none"}))
		}
	}
	return next(ctx)
}

// tokenUsageFromMsg extracts input/output token counts from a model response
// message. It prefers msg.Usage (message.TokenUsage, non-streaming) and falls
// back to msg.Metadata["usage"] (model.ChatUsage, streaming).
func tokenUsageFromMsg(msg *message.Msg) (input, output int) {
	if msg.Usage != nil {
		return msg.Usage.PromptTokens, msg.Usage.CompletionTokens
	}
	if raw, ok := msg.Metadata["usage"]; ok {
		switch u := raw.(type) {
		case model.ChatUsage:
			return u.PromptTokens, u.CompletionTokens
		case *model.ChatUsage:
			if u != nil {
				return u.PromptTokens, u.CompletionTokens
			}
		}
	}
	return 0, 0
}

// injectBudgetHint appends a hint message to the reasoning input's message
// history, mirroring Python's insertion of a HintBlock before the next
// reasoning step when the budget is exceeded.
func injectBudgetHint(input *ReasoningInput, agentName, hint string) {
	hintMsg := message.NewMsg().
		Role(message.RoleAssistant).
		Name(agentName).
		Content(message.NewHintBlock(hint, "budget")).
		Build()
	input.Messages = append(input.Messages, hintMsg)
}
