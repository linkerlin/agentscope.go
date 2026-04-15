package agent

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// Base provides common fields and lifecycle methods for all agent implementations.
// Concrete agent types should embed *Base to inherit shutdown, usage tracking,
// and hook firing capabilities.
type Base struct {
	ID          string
	Name        string
	Description string
	SysPrompt   string
	Meta        map[string]any

	hooks       []hook.Hook
	streamHooks []hook.StreamHook

	Mu     sync.RWMutex
	Closed bool
	CallWg sync.WaitGroup

	usage atomic.Value // stores model.ChatUsage
}

// NewBase creates a new Base with the given metadata and hooks.
func NewBase(id, name, description, sysPrompt string, meta map[string]any, hooks []hook.Hook, streamHooks []hook.StreamHook) *Base {
	b := &Base{
		ID:          id,
		Name:        name,
		Description: description,
		SysPrompt:   sysPrompt,
		Meta:        meta,
		hooks:       hook.SortByPriority(hooks),
		streamHooks: hook.SortStreamHooks(streamHooks),
	}
	b.usage.Store(model.ChatUsage{})
	return b
}

// AgentName returns the agent's display name.
func (b *Base) AgentName() string { return b.Name }

// Shutdown gracefully closes the agent and waits for ongoing calls to finish.
func (b *Base) Shutdown(ctx context.Context) error {
	b.Mu.Lock()
	b.Closed = true
	b.Mu.Unlock()

	done := make(chan struct{})
	go func() {
		b.CallWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsClosed reports whether the agent has been shut down.
func (b *Base) IsClosed() bool {
	b.Mu.RLock()
	defer b.Mu.RUnlock()
	return b.Closed
}

// TotalUsage returns the accumulated token usage across all calls.
func (b *Base) TotalUsage() model.ChatUsage {
	v, _ := b.usage.Load().(model.ChatUsage)
	return v
}

// AddUsage accumulates the given token usage atomically.
func (b *Base) AddUsage(u model.ChatUsage) {
	for {
		old := b.TotalUsage()
		newU := old.Add(u)
		if b.usage.CompareAndSwap(old, newU) {
			return
		}
	}
}

// FireHooks fires all registered classic hooks for the given point.
// It supports InjectMessages chain-updates and early interruption.
func (b *Base) FireHooks(
	ctx context.Context,
	point hook.HookPoint,
	messages []*message.Msg,
	response *message.Msg,
	toolName string,
	toolInput map[string]any,
) ([]*message.Msg, *hook.HookResult, error) {
	if len(b.hooks) == 0 {
		return messages, nil, nil
	}
	msgs := messages
	for _, h := range b.hooks {
		hCtx := &hook.HookContext{
			AgentName: b.Name,
			Point:     point,
			Messages:  msgs,
			Response:  response,
			ToolName:  toolName,
			ToolInput: toolInput,
			Metadata:  make(map[string]any),
		}
		result, err := h.OnEvent(ctx, hCtx)
		if err != nil {
			return nil, nil, err
		}
		if result != nil && len(result.InjectMessages) > 0 {
			msgs = result.InjectMessages
		}
		if result != nil && (result.Interrupt || result.Override != nil || result.StopAgent || result.GotoReasoning) {
			return msgs, result, nil
		}
	}
	return msgs, nil, nil
}

// HasStreamHooks reports whether any stream hooks are registered.
func (b *Base) HasStreamHooks() bool {
	return len(b.streamHooks) > 0
}

// FireStreamEvent dispatches the event to all registered stream hooks.
func (b *Base) FireStreamEvent(ctx context.Context, ev hook.Event) (hook.Event, *hook.StreamHookResult, error) {
	for _, h := range b.streamHooks {
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
