package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// thinkingSigStreamModel emits reasoning content on the IsThinking delta path
// and a provider signature on the final chunk.
type thinkingSigStreamModel struct {
	mockChatModel
}

func (m *thinkingSigStreamModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 4)
	ch <- &model.StreamChunk{Delta: "reasoning", IsThinking: true}
	ch <- &model.StreamChunk{Delta: "answer"}
	ch <- &model.StreamChunk{Done: true, ThinkingSignature: "sig-xyz"}
	close(ch)
	return ch, nil
}

// TestRunModelStream_ThinkingSignaturePreserved verifies the streaming
// assembler keeps both the thinking text and its provider signature so the
// thinking block round-trips through history replay (PyV2 #2495).
func TestRunModelStream_ThinkingSignaturePreserved(t *testing.T) {
	a := &ReActAgent{
		Base:          agent.NewBase("sig", "sig", "", "", nil, nil, nil),
		chatModel:     &thinkingSigStreamModel{},
		memory:        memory.NewInMemoryMemory(),
		maxIterations: 3,
		toolMap:       map[string]tool.Tool{},
	}
	out := make(chan event.AgentEvent, 64)
	msg, err := a.runModelStream(context.Background(), nil, nil, 0, false, out, "r1")
	if err != nil {
		t.Fatal(err)
	}
	var thinking *message.ThinkingBlock
	for _, b := range msg.Content {
		if tb, ok := b.(*message.ThinkingBlock); ok {
			thinking = tb
		}
	}
	if thinking == nil {
		t.Fatalf("expected thinking block in response, got %+v", msg.Content)
	}
	if thinking.Thinking != "reasoning" {
		t.Fatalf("unexpected thinking text: %q", thinking.Thinking)
	}
	if thinking.Signature != "sig-xyz" {
		t.Fatalf("expected signature sig-xyz, got %q", thinking.Signature)
	}
	if got := msg.GetTextContent(); got != "answer" {
		t.Fatalf("expected answer text, got %q", got)
	}
}
