package react

import (
	"context"
	"strings"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
)

func (a *ReActAgent) fireStreamEvent(ctx context.Context, ev hook.Event) (hook.Event, *hook.StreamHookResult, error) {
	return a.FireStreamEvent(ctx, ev)
}

//nolint:unparam
func (a *ReActAgent) invokeModelChatStream(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
) (<-chan *model.StreamChunk, error) {
	chain := a.MiddlewareChain()
	if chain == nil || len(chain.ModelCall) == 0 {
		return a.effectiveModel(ctx).ChatStream(ctx, limitImages(history, a.contextConfig.MaxImageNum), chatOpts...)
	}
	input := &middleware.ModelCallInput{
		Messages:  append([]*message.Msg(nil), history...),
		ChatOpts:  append([]model.ChatOption(nil), chatOpts...),
		ModelName: a.effectiveModel(ctx).ModelName(),
	}
	// Stream path: middleware wraps by aggregating streamed output into one message.
	// The final closure reads from input so on_model_call mutations to
	// Messages/ChatOpts reach the actual ChatStream call (AGENTS.md #27).
	wrapped := middleware.ChainModelCall(chain, a.Base, input, func(ctx context.Context) (*message.Msg, error) {
		ch, err := a.effectiveModel(ctx).ChatStream(ctx, limitImages(input.Messages, a.contextConfig.MaxImageNum), input.ChatOpts...)
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
