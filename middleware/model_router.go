package middleware

import (
	"context"

	"github.com/linkerlin/agentscope.go/classifier"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/runcontext"
)

// ChatModelCandidate is one routable model.
type ChatModelCandidate struct {
	// Name is the stable identifier the classifier picks.
	Name string
	// Model is the chat model used when this candidate wins.
	Model model.ChatModel
	// Description tells the classifier when to pick this candidate.
	Description string
}

// ModelRouterInterceptor routes each turn to the best candidate model based on
// a classifier judgment over the latest user message (PyV2 #2758). The chosen
// model travels through the request context so concurrent turns stay isolated,
// and any routing failure falls back to the agent's own model.
type ModelRouterInterceptor struct {
	Base

	// Classifier performs the routing judgment.
	Classifier classifier.Classifier
	// Candidates lists the routable models in preference order.
	Candidates []ChatModelCandidate
	// QuestionName overrides the classifier question name (default "model").
	QuestionName string
	// Instructions overrides the default routing instructions.
	Instructions string
}

// NewModelRouter creates a model router. Later candidates act as fallbacks in
// the prompt, so order them from most to least preferred.
func NewModelRouter(c classifier.Classifier, candidates ...ChatModelCandidate) *ModelRouterInterceptor {
	return &ModelRouterInterceptor{Classifier: c, Candidates: candidates}
}

// OnReply implements ReplyInterceptor.
func (m *ModelRouterInterceptor) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	chosen := m.Route(ctx, latestUserText(input.Messages))
	if chosen == nil {
		return next(ctx)
	}
	return next(runcontext.WithModel(ctx, chosen))
}

// Route returns the model the classifier picked, or nil when routing is
// impossible (no candidates, empty input, classifier failure, unknown choice)
// so callers keep the agent's own model.
func (m *ModelRouterInterceptor) Route(ctx context.Context, state string) model.ChatModel {
	if m == nil || m.Classifier == nil || len(m.Candidates) == 0 {
		return nil
	}
	if state == "" {
		return nil
	}
	names := make([]string, 0, len(m.Candidates))
	criteria := make([]string, 0, len(m.Candidates))
	for _, c := range m.Candidates {
		names = append(names, c.Name)
		criteria = append(criteria, c.Description)
	}
	instructions := m.Instructions
	if instructions == "" {
		instructions = "Pick the chat model best suited to answer the user's input."
	}
	question := classifier.Question{
		Name:         m.questionName(),
		Kind:         classifier.KindChoice,
		Choices:      names,
		Criteria:     criteria,
		Instructions: instructions,
	}
	resp, err := m.Classifier.Classify(ctx, state, []classifier.Question{question})
	if err != nil {
		return nil
	}
	answer, ok := resp.Content[m.questionName()]
	if !ok {
		return nil
	}
	for _, c := range m.Candidates {
		if c.Name == answer.Choice {
			return c.Model
		}
	}
	return nil
}

func (m *ModelRouterInterceptor) questionName() string {
	if m.QuestionName != "" {
		return m.QuestionName
	}
	return "model"
}

// latestUserText returns the text of the most recent user message carrying
// content, or "" when there is none.
func latestUserText(msgs []*message.Msg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] == nil || msgs[i].Role != message.RoleUser {
			continue
		}
		if text := msgs[i].GetTextContent(); text != "" {
			return text
		}
	}
	return ""
}

var _ ReplyInterceptor = (*ModelRouterInterceptor)(nil)
