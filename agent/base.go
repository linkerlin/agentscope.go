package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/interruption"
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

	interruptFlag   atomic.Bool
	interruptMsg    atomic.Value // *message.Msg
	interruptSource atomic.Value // interruption.Source
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
	b.ResetInterrupt()
	return b
}

// AgentName returns the agent's display name.
func (b *Base) AgentName() string { return b.Name }

// Interrupt signals the agent to stop its current execution at the next
// checkpoint. This is safe to call from any goroutine.
func (b *Base) Interrupt() {
	b.interruptFlag.Store(true)
	b.interruptSource.Store(interruption.SourceUser)
}

// InterruptWithMsg signals an interrupt and attaches a user message.
func (b *Base) InterruptWithMsg(msg *message.Msg) {
	b.interruptFlag.Store(true)
	b.interruptSource.Store(interruption.SourceUser)
	if msg != nil {
		b.interruptMsg.Store(msg)
	}
}

// InterruptWithSource signals an interrupt with an explicit source.
func (b *Base) InterruptWithSource(source interruption.Source) {
	b.interruptFlag.Store(true)
	if source == "" {
		source = interruption.SourceSystem
	}
	b.interruptSource.Store(source)
}

// ResetInterrupt clears the interrupt flag and associated state.
// Call this at the beginning of each Call() to prepare for new execution.
func (b *Base) ResetInterrupt() {
	b.interruptFlag.Store(false)
	b.interruptMsg.Store((*message.Msg)(nil))
	b.interruptSource.Store(interruption.SourceUser)
}

// CheckInterrupted returns an error if the agent has been interrupted.
// Subclasses should call this at appropriate checkpoints.
func (b *Base) CheckInterrupted() error {
	if b.interruptFlag.Load() {
		return errors.New("agent execution interrupted")
	}
	return nil
}

// IsInterrupted reports whether an interrupt has been requested.
func (b *Base) IsInterrupted() bool {
	return b.interruptFlag.Load()
}

// CreateInterruptContext builds an InterruptContext from the current state.
func (b *Base) CreateInterruptContext(pending []*message.ToolUseBlock) *interruption.Context {
	src, _ := b.interruptSource.Load().(interruption.Source)
	if src == "" {
		src = interruption.SourceUser
	}
	var userMsg *message.Msg
	if v := b.interruptMsg.Load(); v != nil {
		userMsg = v.(*message.Msg)
	}
	return &interruption.Context{
		Source:           src,
		Timestamp:        time.Now(),
		UserMessage:      userMsg,
		PendingToolCalls: pending,
	}
}

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

// Call wraps the full agent reply lifecycle: pre_reply -> reply -> post_reply.
// It fires PreReply hooks, allows them to modify the input message, then invokes
// reply with the potentially modified message. After reply returns, it fires
// PostReply hooks.
func (b *Base) Call(ctx context.Context, msg *message.Msg, reply func(context.Context, *message.Msg) (*message.Msg, error)) (*message.Msg, error) {
	messages := []*message.Msg{msg}

	// PreReply
	msgs, hr, err := b.FireHooks(ctx, hook.HookPreReply, messages, nil, "", nil)
	if err != nil {
		return nil, err
	}
	if hr != nil && (hr.Interrupt || hr.Override != nil) {
		return hr.Override, nil
	}
	input := msg
	if len(msgs) > 0 {
		input = msgs[0]
	}

	resp, err := reply(ctx, input)
	if err != nil {
		return nil, err
	}

	// PostReply
	if resp != nil {
		b.FireHooks(ctx, hook.HookPostReply, append(msgs, resp), resp, "", nil)
	}
	return resp, nil
}

// Observe wraps the full agent observe lifecycle: pre_observe -> observe -> post_observe.
func (b *Base) Observe(ctx context.Context, msg *message.Msg, observe func(context.Context, *message.Msg) error) error {
	messages := []*message.Msg{msg}

	// PreObserve
	msgs, hr, err := b.FireHooks(ctx, hook.HookPreObserve, messages, nil, "", nil)
	if err != nil {
		return err
	}
	if hr != nil && hr.Interrupt {
		return nil
	}
	input := msg
	if len(msgs) > 0 {
		input = msgs[0]
	}

	if err := observe(ctx, input); err != nil {
		return err
	}

	// PostObserve
	b.FireHooks(ctx, hook.HookPostObserve, msgs, nil, "", nil)
	return nil
}
