package react

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

type mockModel struct {
	name      string
	ch        chan *model.StreamChunk
	chatErr   error
	streamErr error
}

func (m *mockModel) ModelName() string { return m.name }

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("sync").Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return m.ch, nil
}

func TestRunModelStreamChunks(t *testing.T) {
	ch := make(chan *model.StreamChunk, 4)
	ch <- &model.StreamChunk{Delta: "a"}
	ch <- &model.StreamChunk{Delta: "b"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)

	var chunks int
	sh := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		if ev.EventType() == hook.EventReasoningChunk {
			chunks++
		}
		return nil, nil
	})

	a := &ReActAgent{
		Base:          agent.NewBase("", "t", "", "", nil, nil, []hook.StreamHook{sh}),
		chatModel:     &mockModel{name: "m", ch: ch},
		memory:        memory.NewInMemoryMemory(),
		maxIterations: 3,
		toolMap:       map[string]tool.Tool{},
	}
	hist := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}
	msg, err := a.runModel(context.Background(), hist, nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "ab" {
		t.Fatalf("got %q", msg.GetTextContent())
	}
	if chunks != 2 {
		t.Fatalf("chunks=%d", chunks)
	}
}


func TestRunModelChatStreamError(t *testing.T) {
	a := &ReActAgent{
		Base: agent.NewBase("", "t", "", "", nil, nil, []hook.StreamHook{hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) { return nil, nil })}),
		chatModel: &mockModel{name: "m", streamErr: errors.New("stream err")},
		memory:    memory.NewInMemoryMemory(),
	}
	_, err := a.runModel(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}, nil, 0, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunModelChatError(t *testing.T) {
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, nil),
		chatModel: &mockModel{name: "m", chatErr: errors.New("chat err")},
		memory:    memory.NewInMemoryMemory(),
	}
	_, err := a.runModel(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}, nil, 0, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunModelNilChunk(t *testing.T) {
	ch := make(chan *model.StreamChunk, 3)
	ch <- &model.StreamChunk{Delta: "a"}
	ch <- nil
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, []hook.StreamHook{hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) { return nil, nil })}),
		chatModel: &mockModel{name: "m", ch: ch},
		memory:    memory.NewInMemoryMemory(),
	}
	msg, err := a.runModel(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}, nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "a" {
		t.Fatalf("expected a, got %s", msg.GetTextContent())
	}
}
