package middleware_test

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
)

// msgWithUsage builds an assistant message carrying the given token usage on
// msg.Usage (the non-streaming representation).
func msgWithUsage(prompt, completion int) *message.Msg {
	m := message.NewMsg().Role(message.RoleAssistant).TextContent("resp").Build()
	m.Usage = &message.TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
	return m
}

// msgWithStreamUsage builds an assistant message carrying usage in Metadata
// (the streaming representation, model.ChatUsage).
func msgWithStreamUsage(prompt, completion int) *message.Msg {
	m := message.NewMsg().Role(message.RoleAssistant).TextContent("resp").Build()
	m.Metadata["usage"] = model.ChatUsage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
	return m
}

// applyChatOpts applies a slice of ChatOption to a fresh ChatOptions and
// returns it, so tests can inspect the resolved ToolChoice.
func applyChatOpts(opts []model.ChatOption) model.ChatOptions {
	co := model.ChatOptions{}
	for _, o := range opts {
		o(&co)
	}
	return co
}

// hasHintMessage reports whether any message in the slice carries a HintBlock.
func hasHintMessage(msgs []*message.Msg) bool {
	for _, m := range msgs {
		for _, b := range m.Content {
			if _, ok := b.(*message.HintBlock); ok {
				return true
			}
		}
	}
	return false
}

// runBudgetReply simulates a reply that performs len(usages) reasoning steps,
// each issuing one model call with the corresponding usage. It wires the
// budget middleware through ChainReply -> ChainReasoning -> ChainModelCall
// exactly like the real ReAct agent, so the shared context.Context carries the
// per-reply accumulator across hooks.
//
// It returns the resolved ChatOptions.ToolChoice of the LAST reasoning step
// and whether a hint message was present in that step's input messages.
func runBudgetReply(t *testing.T, mw *middleware.BudgetControlMiddleware, usages []msgUsage) (lastToolChoice *model.ToolChoice, lastHint bool) {
	t.Helper()
	chain := middleware.Classify([]middleware.Middleware{mw})
	agent := stubAgent{name: "budget-agent"}

	lastToolChoice = nil
	lastHint = false

	replyHandler := middleware.ChainReply(chain, agent, &middleware.ReplyInput{
		Messages: []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()},
	}, func(ctx context.Context) (*message.Msg, error) {
		for i, u := range usages {
			rInput := &middleware.ReasoningInput{
				Iteration: i,
				Messages:  []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("q").Build()},
				ChatOpts:  nil,
			}
			rHandler := middleware.ChainReasoning(chain, agent, rInput, func(ctx context.Context) (*message.Msg, error) {
				mInput := &middleware.ModelCallInput{
					Messages:  rInput.Messages,
					ChatOpts:  append([]model.ChatOption(nil), rInput.ChatOpts...),
					ModelName: "test-model",
				}
				mHandler := middleware.ChainModelCall(chain, agent, mInput, func(ctx context.Context) (*message.Msg, error) {
					return u.build(u.prompt, u.completion), nil
				})
				return mHandler(ctx)
			})
			if _, err := rHandler(ctx); err != nil {
				t.Fatalf("reasoning step %d: %v", i, err)
			}
			// Snapshot the enforcement state seen by this step's model call.
			resolved := applyChatOpts(rInput.ChatOpts)
			lastToolChoice = resolved.ToolChoice
			lastHint = hasHintMessage(rInput.Messages)
		}
		return message.NewMsg().Role(message.RoleAssistant).TextContent("done").Build(), nil
	})

	if _, err := replyHandler(context.Background()); err != nil {
		t.Fatalf("reply: %v", err)
	}
	return lastToolChoice, lastHint
}

type msgUsage struct {
	prompt, completion int
	build              func(int, int) *message.Msg
}

func TestBudgetControl_NoEnforcementUnderBudget(t *testing.T) {
	mw := middleware.NewBudgetControlMiddleware(1000)
	// Two calls: 10+5=15, then 10+5=15 -> total 30, well under 1000.
	lastTC, lastHint := runBudgetReply(t, mw, []msgUsage{
		{prompt: 10, completion: 5, build: msgWithUsage},
		{prompt: 10, completion: 5, build: msgWithUsage},
	})
	if lastTC != nil {
		t.Fatalf("expected no tool_choice override under budget, got %+v", lastTC)
	}
	if lastHint {
		t.Fatal("expected no hint injection under budget")
	}
}

func TestBudgetControl_EnforcesWhenOverBudget(t *testing.T) {
	// Budget 20. First call costs 10+5=15 (under). Second reasoning step sees
	// accumulated 15 < 20 -> still allowed, no enforcement on step 2's call.
	// After step 2's model call, accumulated = 30 >= 20. A third reasoning
	// step must then enforce (force none + hint).
	mw := middleware.NewBudgetControlMiddleware(20)
	lastTC, lastHint := runBudgetReply(t, mw, []msgUsage{
		{prompt: 10, completion: 5, build: msgWithUsage},
		{prompt: 10, completion: 5, build: msgWithUsage},
		{prompt: 1, completion: 1, build: msgWithUsage},
	})
	if lastTC == nil || lastTC.Mode != "none" {
		t.Fatalf("expected tool_choice=none after budget exhaustion, got %+v", lastTC)
	}
	if !lastHint {
		t.Fatal("expected hint injection after budget exhaustion")
	}
}

func TestBudgetControl_OutputWeighting(t *testing.T) {
	// Budget 12. One call: input=4 (weight 1), output=4 (weight 2) -> cost 4+8=12.
	// After this call, accumulated 12 >= 12, so the NEXT reasoning step enforces.
	mw := middleware.NewBudgetControlMiddleware(12).
		WithInputTokenWeight(1).
		WithOutputTokenWeight(2)
	lastTC, _ := runBudgetReply(t, mw, []msgUsage{
		{prompt: 4, completion: 4, build: msgWithUsage},
		{prompt: 1, completion: 1, build: msgWithUsage},
	})
	if lastTC == nil || lastTC.Mode != "none" {
		t.Fatalf("expected enforcement with weighted cost, got %+v", lastTC)
	}
}

func TestBudgetControl_StreamUsageFallback(t *testing.T) {
	// Streaming path stores usage in Metadata["usage"]; the middleware must
	// still accumulate it. Budget 18; call cost 10+8=18 -> next step enforces.
	mw := middleware.NewBudgetControlMiddleware(18)
	lastTC, lastHint := runBudgetReply(t, mw, []msgUsage{
		{prompt: 10, completion: 8, build: msgWithStreamUsage},
		{prompt: 1, completion: 1, build: msgWithStreamUsage},
	})
	if lastTC == nil || lastTC.Mode != "none" {
		t.Fatalf("expected enforcement from stream usage, got %+v", lastTC)
	}
	if !lastHint {
		t.Fatal("expected hint from stream usage accumulation")
	}
}

func TestBudgetControl_StatelessAcrossReplies(t *testing.T) {
	// The same middleware instance reused for two independent replies must
	// not leak budget between them: a tiny first reply must not cause
	// enforcement in a second tiny reply.
	mw := middleware.NewBudgetControlMiddleware(1_000_000)
	_, hint1 := runBudgetReply(t, mw, []msgUsage{
		{prompt: 100, completion: 100, build: msgWithUsage},
	})
	_, hint2 := runBudgetReply(t, mw, []msgUsage{
		{prompt: 100, completion: 100, build: msgWithUsage},
	})
	if hint1 || hint2 {
		t.Fatalf("budget state leaked across replies: hint1=%v hint2=%v", hint1, hint2)
	}
}

func TestBudgetControl_BuilderDefaults(t *testing.T) {
	mw := middleware.NewBudgetControlMiddleware(500)
	if mw.TokenBudget != 500 || mw.InputTokenWeight != 1 || mw.OutputTokenWeight != 1 {
		t.Fatalf("unexpected defaults: %+v", mw)
	}
	if mw.HintMessage != middleware.DefaultBudgetHintMessage {
		t.Fatal("default hint message mismatch")
	}
	mw.WithInputTokenWeight(3).WithOutputTokenWeight(4).WithHintMessage("stop now")
	if mw.InputTokenWeight != 3 || mw.OutputTokenWeight != 4 || mw.HintMessage != "stop now" {
		t.Fatalf("builder setters failed: %+v", mw)
	}
}
