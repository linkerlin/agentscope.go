package classifier

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type fakeModel struct {
	name  string
	reply string
	err   error
}

func (f *fakeModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if f.err != nil {
		return nil, f.err
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(f.reply).Build(), nil
}

func (f *fakeModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 1)
	ch <- &model.StreamChunk{Delta: f.reply}
	close(ch)
	return ch, nil
}

func (f *fakeModel) ModelName() string { return f.name }

func TestChatClassifier_ParsesAllKinds(t *testing.T) {
	m := &fakeModel{
		name: "clf",
		reply: `{
			"is_hard": {"probability": 0.82},
			"topic": {"choice": "code", "confidence": 0.9, "probabilities": {"code": 0.9, "chat": 0.1}},
			"quality": {"score": 4, "confidence": 0.7}
		}`,
	}
	c := NewChatClassifier(m)
	resp, err := c.Classify(context.Background(), "write me a binary search", []Question{
		{Name: "is_hard", Kind: KindBinary},
		{Name: "topic", Kind: KindChoice, Choices: []string{"code", "chat"}},
		{Name: "quality", Kind: KindScore},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "clf" {
		t.Fatalf("unexpected model name: %q", resp.Model)
	}
	if resp.Content["is_hard"].Probability != 0.82 {
		t.Fatalf("unexpected binary answer: %+v", resp.Content["is_hard"])
	}
	if resp.Content["topic"].Choice != "code" || resp.Content["topic"].Confidence != 0.9 {
		t.Fatalf("unexpected choice answer: %+v", resp.Content["topic"])
	}
	if resp.Content["quality"].Score != 4 {
		t.Fatalf("unexpected score answer: %+v", resp.Content["quality"])
	}
}

func TestChatClassifier_StripsCodeFence(t *testing.T) {
	m := &fakeModel{name: "clf", reply: "```json\n{\"q\": {\"probability\": 0.5}}\n```"}
	c := NewChatClassifier(m)
	resp, err := c.Classify(context.Background(), "state", []Question{{Name: "q", Kind: KindBinary}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content["q"].Probability != 0.5 {
		t.Fatalf("unexpected answer: %+v", resp.Content["q"])
	}
}

func TestChatClassifier_ChoiceCaseInsensitive(t *testing.T) {
	m := &fakeModel{name: "clf", reply: `{"q": {"choice": "CODE"}}`}
	c := NewChatClassifier(m)
	resp, err := c.Classify(context.Background(), "state", []Question{
		{Name: "q", Kind: KindChoice, Choices: []string{"code", "chat"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content["q"].Choice != "code" {
		t.Fatalf("expected canonical option, got %q", resp.Content["q"].Choice)
	}
}

func TestChatClassifier_Errors(t *testing.T) {
	c := NewChatClassifier(&fakeModel{name: "clf", reply: `{}`})
	if _, err := c.Classify(context.Background(), "state", []Question{{Name: "q", Kind: KindBinary}}); err == nil {
		t.Fatal("expected missing-answer error")
	}

	c = NewChatClassifier(&fakeModel{name: "clf", reply: `{"q": {"choice": "other"}}`})
	if _, err := c.Classify(context.Background(), "state", []Question{
		{Name: "q", Kind: KindChoice, Choices: []string{"a", "b"}},
	}); err == nil {
		t.Fatal("expected non-option choice error")
	}

	c = NewChatClassifier(&fakeModel{name: "clf", reply: `not json`})
	if _, err := c.Classify(context.Background(), "state", []Question{{Name: "q", Kind: KindBinary}}); err == nil {
		t.Fatal("expected invalid JSON error")
	}

	c = NewChatClassifier(&fakeModel{name: "clf", err: errors.New("boom")})
	if _, err := c.Classify(context.Background(), "state", []Question{{Name: "q", Kind: KindBinary}}); err == nil {
		t.Fatal("expected model error")
	}

	if _, err := (*ChatClassifier)(nil).Classify(context.Background(), "state", nil); err == nil {
		t.Fatal("expected nil classifier error")
	}
	if _, err := NewChatClassifier(&fakeModel{name: "clf"}).Classify(context.Background(), "  ", []Question{{Name: "q", Kind: KindBinary}}); err == nil {
		t.Fatal("expected empty state error")
	}
}

func TestValidateQuestions(t *testing.T) {
	if err := ValidateQuestions([]Question{{Name: "q", Kind: KindBinary}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateQuestions([]Question{{Name: "q", Kind: KindChoice, Choices: []string{"only"}}}); err == nil {
		t.Fatal("expected choice-options error")
	}
	if err := ValidateQuestions([]Question{{Name: "q", Kind: "weird"}}); err == nil {
		t.Fatal("expected unknown-kind error")
	}
	if err := ValidateQuestions([]Question{{Name: "", Kind: KindBinary}}); err == nil {
		t.Fatal("expected empty-name error")
	}
	if err := ValidateQuestions(nil); err == nil {
		t.Fatal("expected empty-questions error")
	}
}

func TestRenderQuestions(t *testing.T) {
	out := renderQuestions([]Question{{
		Name:         "topic",
		Kind:         KindChoice,
		Choices:      []string{"code", "chat"},
		Instructions: "prefer code when unsure",
	}})
	for _, want := range []string{"topic", "code | chat", "prefer code when unsure"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in prompt:\n%s", want, out)
		}
	}
}
