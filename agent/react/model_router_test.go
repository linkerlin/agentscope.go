package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/classifier"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
)

// tagModel tags every reply with its own name so routing is observable.
type tagModel struct{ tag string }

func (m *tagModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.tag).Build(), nil
}

func (m *tagModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: m.tag}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *tagModel) ModelName() string { return m.tag }

type tagClassifier struct{ choice string }

func (c *tagClassifier) Classify(ctx context.Context, state string, questions []classifier.Question) (*classifier.Response, error) {
	return &classifier.Response{
		Model: "stub",
		Content: map[string]classifier.Answer{
			"model": {Name: "model", Kind: classifier.KindChoice, Choice: c.choice},
		},
	}, nil
}

func routedAgent(t *testing.T, choice string) (*ReActAgent, *tagModel, *tagModel) {
	t.Helper()
	strong := &tagModel{tag: "strong"}
	cheap := &tagModel{tag: "cheap"}
	a, err := Builder().Name("t").Model(strong).Middlewares(middleware.NewModelRouter(
		&tagClassifier{choice: choice},
		middleware.ChatModelCandidate{Name: "cheap", Model: cheap, Description: "simple questions"},
		middleware.ChatModelCandidate{Name: "strong", Model: strong, Description: "hard questions"},
	)).Build()
	if err != nil {
		t.Fatal(err)
	}
	return a, strong, cheap
}

// TestReActAgent_ModelRouter_V1Call verifies routing applies on the
// synchronous path.
func TestReActAgent_ModelRouter_V1Call(t *testing.T) {
	a, _, _ := routedAgent(t, "cheap")
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("say hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "cheap" {
		t.Fatalf("expected routed cheap model, got %q", resp.GetTextContent())
	}
}

// TestReActAgent_ModelRouter_V2Stream verifies routing applies on the
// event-stream path used by gateway and console.
func TestReActAgent_ModelRouter_V2Stream(t *testing.T) {
	a, _, _ := routedAgent(t, "cheap")
	ch, err := a.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("say hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for ev := range ch {
		if d, ok := ev.(*event.TextBlockDeltaEvent); ok {
			text += d.Delta
		}
	}
	if text != "cheap" {
		t.Fatalf("expected routed cheap model on stream path, got %q", text)
	}
}

// TestReActAgent_ModelRouter_FallbackKeepsOwnModel verifies a failed routing
// judgment leaves the agent on its own model.
func TestReActAgent_ModelRouter_FallbackKeepsOwnModel(t *testing.T) {
	a, _, _ := routedAgent(t, "does-not-exist")
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("say hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "strong" {
		t.Fatalf("expected fallback to own model, got %q", resp.GetTextContent())
	}
}
