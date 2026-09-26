package react

import (
	"context"
	"sync"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// usageTrackingModel wraps a ChatModel and accumulates token usage across all
// Chat calls, including calls made deep inside helpers such as the structured
// output runner. It lets background LLM spend (context compression, output
// repair) show up in Base.TotalUsage instead of vanishing (E8b).
type usageTrackingModel struct {
	model.ChatModel
	mu    sync.Mutex
	total model.ChatUsage
}

func (m *usageTrackingModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	resp, err := m.ChatModel.Chat(ctx, messages, options...)
	if err == nil && resp != nil {
		if u, ok := resp.Metadata["usage"].(model.ChatUsage); ok {
			m.mu.Lock()
			m.total = m.total.Add(u)
			m.mu.Unlock()
		}
	}
	return resp, err
}

func (m *usageTrackingModel) totalUsage() model.ChatUsage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.total
}

// addResponseUsage records the usage attached to a direct Chat response.
func addResponseUsage(add func(model.ChatUsage), resp *message.Msg) {
	if resp == nil {
		return
	}
	if u, ok := resp.Metadata["usage"].(model.ChatUsage); ok {
		add(u)
	}
}

// tokenUsageFrom converts accumulated chat usage to a message-level usage record.
func tokenUsageFrom(u model.ChatUsage) *message.TokenUsage {
	return &message.TokenUsage{
		PromptTokens:        u.PromptTokens,
		CompletionTokens:    u.CompletionTokens,
		TotalTokens:         u.TotalTokens,
		CachedPromptTokens:  u.CachedPromptTokens,
		CacheCreationTokens: u.CacheCreationTokens,
	}
}

// withCurrentUsage stamps a synthetic terminal message (interruption recovery,
// breaker notice, denial notice) with the agent's accumulated usage so every
// final message stays observable (E8c).
func (a *ReActAgent) withCurrentUsage(msg *message.Msg) *message.Msg {
	if msg == nil {
		return nil
	}
	msg.Usage = tokenUsageFrom(a.TotalUsage())
	return msg
}
