package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/classifier"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/runcontext"
)

type routerFakeModel struct{ name string }

func (f *routerFakeModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(f.name).Build(), nil
}

func (f *routerFakeModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 1)
	ch <- &model.StreamChunk{Delta: f.name}
	close(ch)
	return ch, nil
}

func (f *routerFakeModel) ModelName() string { return f.name }

type fakeClassifier struct {
	choice string
	err    error
	state  string
}

func (f *fakeClassifier) Classify(ctx context.Context, state string, questions []classifier.Question) (*classifier.Response, error) {
	f.state = state
	if f.err != nil {
		return nil, f.err
	}
	return &classifier.Response{
		Content: map[string]classifier.Answer{
			"model": {Name: "model", Kind: classifier.KindChoice, Choice: f.choice},
		},
	}, nil
}

func routerInput(text string) *ReplyInput {
	return &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(text).Build(),
	}}
}

func TestModelRouter_SelectsCandidate(t *testing.T) {
	cheap := &routerFakeModel{name: "cheap"}
	strong := &routerFakeModel{name: "strong"}
	mw := NewModelRouter(&fakeClassifier{choice: "cheap"},
		ChatModelCandidate{Name: "cheap", Model: cheap, Description: "simple questions"},
		ChatModelCandidate{Name: "strong", Model: strong, Description: "hard questions"},
	)

	var seen model.ChatModel
	next := func(ctx context.Context) (*message.Msg, error) {
		seen = runcontext.Model(ctx)
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}
	if _, err := mw.OnReply(context.Background(), nil, routerInput("say hi"), next); err != nil {
		t.Fatal(err)
	}
	if seen == nil || seen.ModelName() != "cheap" {
		t.Fatalf("expected cheap model in context, got %v", seen)
	}
}

func TestModelRouter_FallsBackOnErrors(t *testing.T) {
	cheap := &routerFakeModel{name: "cheap"}
	cases := []struct {
		name string
		mw   *ModelRouterInterceptor
	}{
		{"classifier error", NewModelRouter(&fakeClassifier{err: errors.New("boom")}, ChatModelCandidate{Name: "cheap", Model: cheap})},
		{"unknown choice", NewModelRouter(&fakeClassifier{choice: "nope"}, ChatModelCandidate{Name: "cheap", Model: cheap})},
		{"no candidates", NewModelRouter(&fakeClassifier{choice: "cheap"})},
		{"no classifier", NewModelRouter(nil, ChatModelCandidate{Name: "cheap", Model: cheap})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen model.ChatModel
			next := func(ctx context.Context) (*message.Msg, error) {
				seen = runcontext.Model(ctx)
				return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
			}
			if _, err := tc.mw.OnReply(context.Background(), nil, routerInput("hi"), next); err != nil {
				t.Fatal(err)
			}
			if seen != nil {
				t.Fatalf("expected fallback (nil override), got %v", seen)
			}
		})
	}
}

func TestModelRouter_EmptyInputSkipsRouting(t *testing.T) {
	fc := &fakeClassifier{choice: "cheap"}
	mw := NewModelRouter(fc, ChatModelCandidate{Name: "cheap", Model: &routerFakeModel{name: "cheap"}})
	if got := mw.Route(context.Background(), ""); got != nil {
		t.Fatalf("expected nil route for empty input, got %v", got)
	}
	if fc.state != "" {
		t.Fatal("classifier must not be called for empty input")
	}
}

func TestLatestUserText(t *testing.T) {
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("first").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("answer").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("latest").Build(),
	}
	if got := latestUserText(msgs); got != "latest" {
		t.Fatalf("unexpected latest user text: %q", got)
	}
	if got := latestUserText(nil); got != "" {
		t.Fatalf("expected empty for nil messages, got %q", got)
	}
}
