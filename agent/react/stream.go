package react

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func (a *ReActAgent) fireStreamEvent(ctx context.Context, ev hook.Event) (hook.Event, *hook.StreamHookResult, error) {
	for _, h := range a.streamHooks {
		r, err := h.OnStreamEvent(ctx, ev)
		if err != nil {
			return ev, nil, err
		}
		if r != nil && r.Interrupt {
			return ev, r, hook.ErrInterrupted
		}
	}
	return ev, nil, nil
}

// runModel 执行一次模型调用：在注册 StreamHook 且本轮未声明工具时走 ChatStream 并派发 chunk；否则走 Chat（保证 tool call 正确）
func (a *ReActAgent) runModel(
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
			Agent:     a.name,
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

	useStream := len(a.streamHooks) > 0 && !requestTools

	if !useStream {
		msg, err := a.chatModel.Chat(ctx, history, chatOpts...)
		if err != nil {
			_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.name, Iteration: iter},
				Err:       err,
			})
			return nil, fmt.Errorf("react agent model call: %w", err)
		}
		_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.name, Iteration: iter},
			Messages:  append([]*message.Msg(nil), history...),
			Response:  msg,
		})
		return msg, nil
	}

	ch, err := a.chatModel.ChatStream(ctx, history, chatOpts...)
	if err != nil {
		_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.name, Iteration: iter},
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
				BaseEvent: hook.BaseEvent{Type: hook.EventReasoningChunk, Ts: time.Now(), Agent: a.name, Iteration: iter},
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
		BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.name, Iteration: iter},
		Messages:  append([]*message.Msg(nil), history...),
		Response:  msg,
	})
	return msg, nil
}
