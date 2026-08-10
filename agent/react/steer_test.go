package react

import (
	"context"
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
