package react

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
)

func (a *ReActAgent) fireStreamEvent(ctx context.Context, ev hook.Event) (hook.Event, *hook.StreamHookResult, error) {
	return a.Base.FireStreamEvent(ctx, ev)
}

// runModel 执行一次模型调用：在注册 StreamHook 且本轮未声明工具时走 ChatStream 并派发 chunk；否则走 Chat（保证 tool call 正确）
func (a *ReActAgent) runModel(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
	requestTools bool,
) (*message.Msg, error) {
	chain := a.Base.MiddlewareChain()
	if chain == nil || len(chain.Reasoning) == 0 {
		return a.runModelInner(ctx, history, chatOpts, iter, requestTools)
	}
	// Build the reasoning input up front so the final closure reads from it.
	// This lets on_reasoning middleware mutate Messages (e.g. inject budget
	// hints) and ChatOpts (e.g. force tool_choice=none) and have the changes
	// take effect on the actual model call — aligning with Python v2
	// MiddlewareBase semantics.
	input := &middleware.ReasoningInput{
		Iteration: iter,
		Messages:  append([]*message.Msg(nil), history...),
		ChatOpts:  append([]model.ChatOption(nil), chatOpts...),
	}
	handler := middleware.ChainReasoning(chain, a.Base, input, func(ctx context.Context) (*message.Msg, error) {
		return a.runModelInner(ctx, input.Messages, input.ChatOpts, iter, requestTools)
	})
	return handler(ctx)
}

func (a *ReActAgent) runModelInner(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
	requestTools bool,
) (*message.Msg, error) {
	now := time.Now()
	pre := &hook.PreReasoningEvent{
		BaseEvent: hook.BaseEvent{
			Type:      hook.EventPreReasoning,
			Ts:        now,
			Agent:     a.Base.Name,
			Iteration: iter,
		},
		Messages:  append([]*message.Msg(nil), history...),
		ModelName: a.effectiveModel(ctx).ModelName(),
		ChatOpts:  chatOpts,
	}
	if ev, _, err := a.fireStreamEvent(ctx, pre); err != nil {
		return nil, err
	} else if preEv, ok := ev.(*hook.PreReasoningEvent); ok {
		chatOpts = preEv.ChatOpts
	}

	useStream := a.Base.HasStreamHooks() && !requestTools

	if !useStream {
		msg, err := a.invokeModelChat(ctx, history, chatOpts, iter)
		if err != nil {
			_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
				Err:       err,
			})
			return nil, fmt.Errorf("react agent model call: %w", err)
		}
		if isDegenerateResponse(msg) {
			_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
				Err:       errEmptyModelResponse,
			})
			return nil, errEmptyModelResponse
		}
		_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
			Messages:  append([]*message.Msg(nil), history...),
			Response:  msg,
		})
		return msg, nil
	}

	ch, err := a.invokeModelChatStream(ctx, history, chatOpts, iter)
	if err != nil {
		_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
			Err:       err,
		})
		return nil, fmt.Errorf("react agent model stream: %w", err)
	}
	if ch == nil {
		return nil, errEmptyModelResponse
	}
	defer drainStreamAsync(ch)
	var sb strings.Builder
	var thinkingSb strings.Builder
	var thinkSig string
	var streamUsage *model.ChatUsage
	for chunk := range ch {
		if chunk == nil {
			continue
		}
		// Stop consuming as soon as an interrupt or cancellation arrives
		// instead of reading the whole stream first (E4b). The deferred
		// drain lets the producer finish in the background.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if err := a.CheckInterrupted(); err != nil {
			return nil, err
		}
		if chunk.Done {
			if chunk.Error != nil {
				_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
					BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
					Err:       chunk.Error,
				})
				return nil, fmt.Errorf("react agent model stream: %w", chunk.Error)
			}
			if chunk.Usage != nil {
				streamUsage = chunk.Usage
			}
			// Preserve the accumulated reasoning signature for history replay
			// (PyV2 #2495).
			if chunk.ThinkingSignature != "" {
				thinkSig = chunk.ThinkingSignature
			}
			break
		}
		if chunk.Delta != "" {
			// Keep reasoning content out of the answer text so thinking-only
			// streams do not masquerade as text (pairs with hasOnlyThinkingBlocks).
			if chunk.IsThinking {
				thinkingSb.WriteString(chunk.Delta)
			} else {
				sb.WriteString(chunk.Delta)
			}
			if _, _, err := a.fireStreamEvent(ctx, &hook.ReasoningChunkEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventReasoningChunk, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
				Messages:  append([]*message.Msg(nil), history...),
				Chunk:     chunk.Delta,
			}); err != nil {
				if errors.Is(err, hook.ErrInterrupted) {
					return nil, hook.ErrInterrupted
				}
				return nil, err
			}
		}
	}
	var content []message.ContentBlock
	if sb.Len() > 0 {
		content = append(content, message.NewTextBlock(sb.String()))
	}
	if thinkingSb.Len() > 0 {
		content = append(content, message.NewThinkingBlock(thinkingSb.String(), thinkSig))
	}
	msg := message.NewMsg().Role(message.RoleAssistant).Content(content...).Build()
	if isDegenerateResponse(msg) {
		return nil, errEmptyModelResponse
	}
	if streamUsage != nil {
		msg.Metadata["usage"] = *streamUsage
	}
	_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
		Messages:  append([]*message.Msg(nil), history...),
		Response:  msg,
	})
	return msg, nil
}

//nolint:unparam
func (a *ReActAgent) invokeModelChat(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
) (*message.Msg, error) {
	chain := a.Base.MiddlewareChain()
	if chain == nil || len(chain.ModelCall) == 0 {
		return a.effectiveModel(ctx).Chat(ctx, history, chatOpts...)
	}
	input := &middleware.ModelCallInput{
		Messages:  append([]*message.Msg(nil), history...),
		ChatOpts:  append([]model.ChatOption(nil), chatOpts...),
		ModelName: a.effectiveModel(ctx).ModelName(),
	}
	// Final closure reads from input so on_model_call middleware can mutate
	// Messages/ChatOpts and affect the actual Chat call.
	handler := middleware.ChainModelCall(chain, a.Base, input, func(ctx context.Context) (*message.Msg, error) {
		return a.effectiveModel(ctx).Chat(ctx, input.Messages, input.ChatOpts...)
	})
	return handler(ctx)
}

//nolint:unparam
func (a *ReActAgent) invokeModelChatStream(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
) (<-chan *model.StreamChunk, error) {
	final := func(ctx context.Context) (<-chan *model.StreamChunk, error) {
		return a.effectiveModel(ctx).ChatStream(ctx, history, chatOpts...)
	}
	chain := a.Base.MiddlewareChain()
	if chain == nil || len(chain.ModelCall) == 0 {
		return final(ctx)
	}
	input := &middleware.ModelCallInput{
		Messages:  append([]*message.Msg(nil), history...),
		ChatOpts:  chatOpts,
		ModelName: a.effectiveModel(ctx).ModelName(),
	}
	// Stream path: middleware wraps by aggregating streamed output into one message.
	wrapped := middleware.ChainModelCall(chain, a.Base, input, func(ctx context.Context) (*message.Msg, error) {
		ch, err := final(ctx)
		if err != nil {
			return nil, err
		}
		var sb strings.Builder
		for chunk := range ch {
			if chunk == nil {
				continue
			}
			if chunk.Done {
				if chunk.Error != nil {
					return nil, chunk.Error
				}
				break
			}
			if chunk.Delta != "" {
				sb.WriteString(chunk.Delta)
			}
		}
		return message.NewMsg().Role(message.RoleAssistant).TextContent(sb.String()).Build(), nil
	})
	msg, err := wrapped(ctx)
	if err != nil {
		return nil, err
	}
	// Re-emit as a single-chunk stream for downstream consumers.
	out := make(chan *model.StreamChunk, 2)
	go func() {
		defer close(out)
		text := msg.GetTextContent()
		if text != "" {
			out <- &model.StreamChunk{Delta: text}
		}
		out <- &model.StreamChunk{Done: true}
	}()
	return out, nil
}

// drainStreamAsync consumes any remaining chunks in the background so the
// model producer can finish and release its HTTP body when this consumer
// abandons the stream early (hook interruption, downstream error) — E5d.
func drainStreamAsync(ch <-chan *model.StreamChunk) {
	if ch == nil {
		return
	}
	go func() {
		for range ch { //nolint:revive // intentional drain to unblock producer
		}
	}()
}
