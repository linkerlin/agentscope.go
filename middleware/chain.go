package middleware

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// Chain holds middleware grouped by implemented hook type.
type Chain struct {
	Reply        []ReplyInterceptor
	Reasoning    []ReasoningInterceptor
	Acting       []ActingInterceptor
	ModelCall    []ModelCallInterceptor
	SystemPrompt []SystemPromptTransformer
}

// Classify groups middleware by the optional interfaces they implement.
func Classify(mws []Middleware) *Chain {
	c := &Chain{}
	for _, mw := range mws {
		if mw == nil {
			continue
		}
		if r, ok := mw.(ReplyInterceptor); ok {
			c.Reply = append(c.Reply, r)
		}
		if r, ok := mw.(ReasoningInterceptor); ok {
			c.Reasoning = append(c.Reasoning, r)
		}
		if r, ok := mw.(ActingInterceptor); ok {
			c.Acting = append(c.Acting, r)
		}
		if r, ok := mw.(ModelCallInterceptor); ok {
			c.ModelCall = append(c.ModelCall, r)
		}
		if r, ok := mw.(SystemPromptTransformer); ok {
			c.SystemPrompt = append(c.SystemPrompt, r)
		}
	}
	return c
}

// ApplySystemPrompt runs on_system_prompt transformers sequentially.
func ApplySystemPrompt(ctx context.Context, agent Agent, chain *Chain, prompt string) (string, error) {
	if chain == nil || len(chain.SystemPrompt) == 0 {
		return prompt, nil
	}
	out := prompt
	for _, mw := range chain.SystemPrompt {
		next, err := mw.OnSystemPrompt(ctx, agent, out)
		if err != nil {
			return "", err
		}
		out = next
	}
	return out, nil
}

func chainReply(
	mws []ReplyInterceptor,
	agent Agent,
	input *ReplyInput,
	final ReplyNext,
) ReplyNext {
	if len(mws) == 0 {
		return final
	}
	var build func(int) ReplyNext
	build = func(i int) ReplyNext {
		if i >= len(mws) {
			return final
		}
		mw := mws[i]
		next := build(i + 1)
		return func(ctx context.Context) (*message.Msg, error) {
			return mw.OnReply(ctx, agent, input, next)
		}
	}
	return build(0)
}

// ChainReply builds an onion chain for on_reply middleware.
func ChainReply(chain *Chain, agent Agent, input *ReplyInput, final ReplyNext) ReplyNext {
	if chain == nil {
		return final
	}
	return chainReply(chain.Reply, agent, input, final)
}

func chainReasoning(
	mws []ReasoningInterceptor,
	agent Agent,
	input *ReasoningInput,
	final ReasoningNext,
) ReasoningNext {
	if len(mws) == 0 {
		return final
	}
	var build func(int) ReasoningNext
	build = func(i int) ReasoningNext {
		if i >= len(mws) {
			return final
		}
		mw := mws[i]
		next := build(i + 1)
		return func(ctx context.Context) (*message.Msg, error) {
			return mw.OnReasoning(ctx, agent, input, next)
		}
	}
	return build(0)
}

// ChainReasoning builds an onion chain for on_reasoning middleware.
func ChainReasoning(chain *Chain, agent Agent, input *ReasoningInput, final ReasoningNext) ReasoningNext {
	if chain == nil {
		return final
	}
	return chainReasoning(chain.Reasoning, agent, input, final)
}

func chainActing(
	mws []ActingInterceptor,
	agent Agent,
	input *ActingInput,
	final ActingNext,
) ActingNext {
	if len(mws) == 0 {
		return final
	}
	var build func(int) ActingNext
	build = func(i int) ActingNext {
		if i >= len(mws) {
			return final
		}
		mw := mws[i]
		next := build(i + 1)
		return func(ctx context.Context) (*tool.Response, error) {
			return mw.OnActing(ctx, agent, input, next)
		}
	}
	return build(0)
}

// ChainActing builds an onion chain for on_acting middleware.
func ChainActing(chain *Chain, agent Agent, input *ActingInput, final ActingNext) ActingNext {
	if chain == nil {
		return final
	}
	return chainActing(chain.Acting, agent, input, final)
}

func chainModelCall(
	mws []ModelCallInterceptor,
	agent Agent,
	input *ModelCallInput,
	final ModelCallNext,
) ModelCallNext {
	if len(mws) == 0 {
		return final
	}
	var build func(int) ModelCallNext
	build = func(i int) ModelCallNext {
		if i >= len(mws) {
			return final
		}
		mw := mws[i]
		next := build(i + 1)
		return func(ctx context.Context) (*message.Msg, error) {
			return mw.OnModelCall(ctx, agent, input, next)
		}
	}
	return build(0)
}

// ChainModelCall builds an onion chain for on_model_call middleware.
func ChainModelCall(chain *Chain, agent Agent, input *ModelCallInput, final ModelCallNext) ModelCallNext {
	if chain == nil {
		return final
	}
	return chainModelCall(chain.ModelCall, agent, input, final)
}
