package react

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// steerMockModel blocks on the first call until released, capturing the
// history of every call so tests can assert steered messages arrived.
type steerMockModel struct {
	firstEntered chan struct{}
	release      chan struct{}
	callCount    int
	histories    [][]*message.Msg
}

func (m *steerMockModel) ModelName() string { return "steer-mock" }

func (m *steerMockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	msg, err := m.Chat(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	ch := make(chan *model.StreamChunk, 4)
	if text := msg.GetTextContent(); text != "" {
		ch <- &model.StreamChunk{Delta: text}
	}
	content := msg.Content
	if len(content) > 0 {
		ch <- &model.StreamChunk{Done: true, Content: content}
	} else {
		ch <- &model.StreamChunk{Done: true}
	}
	close(ch)
	return ch, nil
}

func (m *steerMockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.callCount++
	m.histories = append(m.histories, messages)
	if m.callCount == 1 {
		close(m.firstEntered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return message.NewMsg().Role(message.RoleAssistant).
			Content(message.NewToolUseBlock("tc1", "mock_tool", map[string]any{})).
			Build(), nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("final").Build(), nil
}

func newSteerAgent(t *testing.T, m *steerMockModel) (*ReActAgent, error) {
	t.Helper()
	return Builder().
		Name("steer-test").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(&mockTool{name: "mock_tool", result: "ok"}).
		MaxIterations(5).
		Build()
}

func TestReActAgent_Steer_VisibleInSameTurn(t *testing.T) {
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := newSteerAgent(t, m)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	evCh, err := agent.ReplyStream(ctx, message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err != nil {
		t.Fatal(err)
	}

	<-m.firstEntered // 第一轮模型调用进行中
	if !agent.ActiveTurn() {
		t.Fatal("ActiveTurn must be true while a turn is running")
	}
	if err := agent.Steer("mid-turn message"); err != nil {
		t.Fatalf("steer during active turn: %v", err)
	}
	close(m.release)

	for ev := range evCh {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			t.Fatalf("turn error: %s", e.Err)
		}
	}

	if m.callCount < 2 {
		t.Fatalf("expected at least 2 model calls, got %d", m.callCount)
	}
	// 第二轮调用的历史必须包含被 steer 注入的用户消息。
	second := m.histories[1]
	var found bool
	for _, msg := range second {
		if msg.Role == message.RoleUser {
			for _, c := range msg.Content {
				if tb, ok := c.(*message.TextBlock); ok && tb.Text == "mid-turn message" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("steered message must appear in the model history of the same turn")
	}
	if agent.ActiveTurn() {
		t.Fatal("ActiveTurn must be false after the turn completes")
	}
}

func TestReActAgent_Steer_NoActiveTurn(t *testing.T) {
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := newSteerAgent(t, m)
	if err != nil {
		t.Fatal(err)
	}
	if agent.ActiveTurn() {
		t.Fatal("no turn should be active before any reply")
	}
	if err := agent.Steer("nobody home"); err == nil {
		t.Fatal("steer without an active turn must fail")
	}
}

func TestReActAgent_AbortMidTurn(t *testing.T) {
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := newSteerAgent(t, m)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	evCh, err := agent.ReplyStream(ctx, message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err != nil {
		t.Fatal(err)
	}
	<-m.firstEntered
	cancel() // abort 生效：turn 立即终止

	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-evCh:
			if !ok {
				if agent.ActiveTurn() {
					t.Fatal("aborted turn must not remain active")
				}
				return
			}
		case <-deadline:
			t.Fatal("aborted turn did not terminate in time")
		}
	}
}

func TestReActAgent_ToolResultLabels(t *testing.T) {
	// 开启标签：第二轮模型调用的历史里工具结果消息带 [tool_result:mock_tool]。
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := Builder().
		Name("labels-test").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(&mockTool{name: "mock_tool", result: "tool-out"}).
		MaxIterations(5).
		WithToolResultLabels(true).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	evCh, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	if err != nil {
		t.Fatal(err)
	}
	<-m.firstEntered
	close(m.release)
	for ev := range evCh {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			t.Fatalf("turn error: %s", e.Err)
		}
	}

	// 第二轮（最终轮）历史应含带来源标签的工具结果。
	if len(m.histories) < 2 {
		t.Fatalf("expected >=2 model calls, got %d", len(m.histories))
	}
	var found bool
	for _, msg := range m.histories[1] {
		if msg.Role != message.RoleTool {
			continue
		}
		for _, c := range msg.Content {
			if tb, ok := c.(*message.TextBlock); ok && strings.Contains(tb.Text, "[tool_result:mock_tool]") {
				found = true
			}
			// 标签在 ToolResultBlock 内部。
			if tr, ok := c.(*message.ToolResultBlock); ok {
				for _, inner := range tr.Content {
					if itb, ok := inner.(*message.TextBlock); ok && strings.Contains(itb.Text, "[tool_result:mock_tool]") {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("tool result must carry the provenance header when enabled")
	}
}

func TestReActAgent_ToolResultScreener(t *testing.T) {
	// 审查钩子拒绝 mock_tool 的输出 → 历史中工具结果被隔离占位文本替换。
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := Builder().
		Name("screen-test").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(&mockTool{name: "mock_tool", result: "evil-injection"}).
		MaxIterations(5).
		WithToolResultScreener(func(ctx context.Context, toolName, text string) bool {
			return !strings.Contains(text, "evil")
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	evCh, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	if err != nil {
		t.Fatal(err)
	}
	<-m.firstEntered
	close(m.release)
	for ev := range evCh {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			t.Fatalf("turn error: %s", e.Err)
		}
	}

	if len(m.histories) < 2 {
		t.Fatalf("expected >=2 model calls, got %d", len(m.histories))
	}
	var quarantined, leaked bool
	for _, msg := range m.histories[1] {
		if msg.Role != message.RoleTool {
			continue
		}
		for _, c := range msg.Content {
			if tr, ok := c.(*message.ToolResultBlock); ok {
				for _, inner := range tr.Content {
					if itb, ok := inner.(*message.TextBlock); ok {
						if strings.Contains(itb.Text, "evil-injection") {
							leaked = true
						}
						if strings.Contains(itb.Text, QuarantinePlaceholder) {
							quarantined = true
						}
					}
				}
			}
		}
	}
	if leaked {
		t.Fatal("rejected tool output must not reach history")
	}
	if !quarantined {
		t.Fatal("rejected tool output must be replaced by the quarantine placeholder")
	}
}

func TestReActAgent_TurnWallClockCap(t *testing.T) {
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := Builder().
		Name("wallclock-test").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(&mockTool{name: "mock_tool", result: "ok"}).
		MaxIterations(20).
		MaxTurnDuration(100 * time.Millisecond). // 墙钟上限：第一轮模型调用尚未放行即到期
		Build()
	if err != nil {
		t.Fatal(err)
	}

	evCh, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err != nil {
		t.Fatal(err)
	}
	<-m.firstEntered

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-evCh:
			if !ok {
				if agent.ActiveTurn() {
					t.Fatal("capped turn must not remain active")
				}
				return
			}
			if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
				// 墙钟到期以错误事件终结也可接受。
				return
			}
		case <-deadline:
			t.Fatal("wall-clock cap did not terminate the turn in time")
		}
	}
}

func TestReActAgent_CallHonorsMaxTurnDuration(t *testing.T) {
	// Parity: MaxTurnDuration must cap the synchronous Call path too (the cap
	// was previously applied only to ReplyStream).
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := Builder().
		Name("call-wallclock").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(&mockTool{name: "mock_tool", result: "ok"}).
		MaxIterations(20).
		MaxTurnDuration(100 * time.Millisecond).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = agent.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err == nil {
		t.Fatal("Call must return an error when the wall-clock cap expires mid-turn")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Call ignored MaxTurnDuration: took %v", elapsed)
	}
	if agent.ActiveTurn() {
		t.Fatal("Call path must not leave an active turn")
	}
}

func TestReActAgent_SteerQueueCap(t *testing.T) {
	m := &steerMockModel{firstEntered: make(chan struct{}), release: make(chan struct{})}
	agent, err := newSteerAgent(t, m)
	if err != nil {
		t.Fatal(err)
	}
	evCh, err := agent.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err != nil {
		t.Fatal(err)
	}
	<-m.firstEntered

	for i := 0; i < steerQueueCap; i++ {
		if err := agent.Steer("m"); err != nil {
			t.Fatalf("steer %d: %v", i, err)
		}
	}
	if err := agent.Steer("overflow"); err == nil {
		t.Fatal("steer beyond the queue cap must fail")
	}

	close(m.release)
	for range evCh {
	}
}
