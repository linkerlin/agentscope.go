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
	final := func(ctx context.Context) (*message.Msg, error) {
		return a.runModelInner(ctx, history, chatOpts, iter, requestTools)
	}
	chain := a.Base.MiddlewareChain()
	if chain != nil && len(chain.Reasoning) > 0 {
		input := &middleware.ReasoningInput{
			Iteration: iter,
			Messages:  append([]*message.Msg(nil), history...),
			ChatOpts:  chatOpts,
		}
		handler := middleware.ChainReasoning(chain, a.Base, input, final)
		return handler(ctx)
	}
	return final(ctx)
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
		ModelName: a.chatModel.ModelName(),
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
	var sb strings.Builder
	var streamUsage *model.ChatUsage
	for chunk := range ch {
		if chunk == nil {
			continue
		}
		if chunk.Done {
			if chunk.Usage != nil {
				streamUsage = chunk.Usage
			}
			break
		}
		if chunk.Delta != "" {
			sb.WriteString(chunk.Delta)
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
	msg := message.NewMsg().Role(message.RoleAssistant).TextContent(sb.String()).Build()
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

func (a *ReActAgent) invokeModelChat(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
) (*message.Msg, error) {
	final := func(ctx context.Context) (*message.Msg, error) {
		return a.chatModel.Chat(ctx, history, chatOpts...)
	}
	chain := a.Base.MiddlewareChain()
	if chain == nil || len(chain.ModelCall) == 0 {
		return final(ctx)
	}
	input := &middleware.ModelCallInput{
		Messages:  append([]*message.Msg(nil), history...),
		ChatOpts:  chatOpts,
		ModelName: a.chatModel.ModelName(),
	}
	handler := middleware.ChainModelCall(chain, a.Base, input, final)
	return handler(ctx)
}

func (a *ReActAgent) invokeModelChatStream(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
) (<-chan *model.StreamChunk, error) {
	final := func(ctx context.Context) (<-chan *model.StreamChunk, error) {
		return a.chatModel.ChatStream(ctx, history, chatOpts...)
	}
	chain := a.Base.MiddlewareChain()
	if chain == nil || len(chain.ModelCall) == 0 {
		return final(ctx)
	}
	input := &middleware.ModelCallInput{
		Messages:  append([]*message.Msg(nil), history...),
		ChatOpts:  chatOpts,
		ModelName: a.chatModel.ModelName(),
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
