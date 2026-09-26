// Package classifier provides probability-style classifiers over shared input
// state. A model router uses it to pick a chat model per turn (PyV2 #2758).
package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// Kind identifies a question's answer shape.
type Kind string

const (
	// KindBinary answers with P(true).
	KindBinary Kind = "binary"
	// KindChoice picks one option from Choices.
	KindChoice Kind = "choice"
	// KindScore rates on an integer scale.
	KindScore Kind = "score"
)

// Question is one named classification task.
type Question struct {
	// Name is the stable identifier used in responses.
	Name string
	// Kind selects the answer shape.
	Kind Kind
	// Instructions is optional extra guidance for the classifier model.
	Instructions string
	// Choices lists the options for KindChoice.
	Choices []string
	// Criteria describes the score dimensions (KindScore) or per-option
	// decision criteria (KindChoice, aligned with the option order).
	Criteria []string
}

// Answer is the classifier's judgment for one question.
type Answer struct {
	Name string
	Kind Kind

	// Probability is P(true) for KindBinary.
	Probability float64
	// Choice is the selected option for KindChoice.
	Choice string
	// Score is the rating for KindScore.
	Score int
	// Confidence is the classifier's self-reported confidence (0-1).
	Confidence float64
	// Probabilities carries per-option probabilities for KindChoice.
	Probabilities map[string]float64
}

// Response is one classifier invocation over shared state.
type Response struct {
	// Model names the classifier backend (e.g. chat model name).
	Model string
	// Content maps question names to answers.
	Content map[string]Answer
}

// Classifier rates a set of named questions over shared input state in one
// call.
type Classifier interface {
	Classify(ctx context.Context, state string, questions []Question) (*Response, error)
}

// ValidateQuestions rejects malformed question sets.
func ValidateQuestions(questions []Question) error {
	if len(questions) == 0 {
		return fmt.Errorf("classifier: at least one question is required")
	}
	seen := map[string]bool{}
	for i, q := range questions {
		if strings.TrimSpace(q.Name) == "" {
			return fmt.Errorf("classifier: question %d has an empty name", i+1)
		}
		if seen[q.Name] {
			return fmt.Errorf("classifier: duplicate question name %q", q.Name)
		}
		seen[q.Name] = true
		switch q.Kind {
		case KindBinary, KindScore:
		case KindChoice:
			if len(q.Choices) < 2 {
				return fmt.Errorf("classifier: choice question %q needs at least 2 options", q.Name)
			}
		default:
			return fmt.Errorf("classifier: question %q has unknown kind %q", q.Name, q.Kind)
		}
	}
	return nil
}

// ChatClassifier implements Classifier with a single chat-model call that
// returns JSON judgments for all questions.
type ChatClassifier struct {
	Model model.ChatModel
	// SystemPrompt optionally overrides the built-in instruction.
	SystemPrompt string
}

// NewChatClassifier creates a chat-model-backed classifier.
func NewChatClassifier(m model.ChatModel) *ChatClassifier {
	return &ChatClassifier{Model: m}
}

const defaultClassifierPrompt = `You are a meticulous classifier. ` +
	`Given the input below, answer every listed question with a probability-style judgment. ` +
	`Respond with a single JSON object only, no markdown fence, keyed by question name:
{"<question>": {"probability": 0.0-1.0} | {"choice": "<option>", "confidence": 0.0-1.0, "probabilities": {"<option>": 0.0-1.0}} | {"score": 1-5, "confidence": 0.0-1.0, "probabilities": {"<score>": 0.0-1.0}}}`

func (c *ChatClassifier) Classify(ctx context.Context, state string, questions []Question) (*Response, error) {
	if c == nil || c.Model == nil {
		return nil, fmt.Errorf("classifier: nil model")
	}
	if err := ValidateQuestions(questions); err != nil {
		return nil, err
	}
	if strings.TrimSpace(state) == "" {
		return nil, fmt.Errorf("classifier: empty input state")
	}

	system := c.SystemPrompt
	if system == "" {
		system = defaultClassifierPrompt
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(system + "\n\nQuestions:\n" + renderQuestions(questions)).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(state).Build(),
	}
	resp, err := c.Model.Chat(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("classifier: model call: %w", err)
	}
	raw := stripCodeFence(resp.GetTextContent())
	var parsed map[string]rawAnswer
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("classifier: invalid classifier output: %w", err)
	}

	out := &Response{Model: c.Model.ModelName(), Content: make(map[string]Answer, len(questions))}
	for _, q := range questions {
		r, ok := parsed[q.Name]
		if !ok {
			return nil, fmt.Errorf("classifier: no answer for question %q", q.Name)
		}
		answer, err := r.toAnswer(q)
		if err != nil {
			return nil, err
		}
		out.Content[q.Name] = answer
	}
	return out, nil
}

func renderQuestions(questions []Question) string {
	var sb strings.Builder
	for i, q := range questions {
		fmt.Fprintf(&sb, "%d. %s (%s)", i+1, q.Name, q.Kind)
		if len(q.Choices) > 0 {
			fmt.Fprintf(&sb, " options: %s", strings.Join(q.Choices, " | "))
		}
		if len(q.Criteria) > 0 {
			fmt.Fprintf(&sb, " criteria: %s", strings.Join(q.Criteria, "; "))
		}
		if q.Instructions != "" {
			fmt.Fprintf(&sb, " note: %s", q.Instructions)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

type rawAnswer struct {
	Probability   *float64           `json:"probability,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Score         *int               `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

func (r rawAnswer) toAnswer(q Question) (Answer, error) {
	a := Answer{Name: q.Name, Kind: q.Kind, Confidence: r.Confidence}
	switch q.Kind {
	case KindBinary:
		if r.Probability == nil {
			return a, fmt.Errorf("classifier: binary question %q missing probability", q.Name)
		}
		a.Probability = *r.Probability
	case KindChoice:
		if strings.TrimSpace(r.Choice) == "" {
			return a, fmt.Errorf("classifier: choice question %q missing choice", q.Name)
		}
		canonical, ok := canonicalChoice(q.Choices, r.Choice)
		if !ok {
			return a, fmt.Errorf("classifier: choice %q is not an option of %q", r.Choice, q.Name)
		}
		a.Choice = canonical
		a.Probabilities = r.Probabilities
	case KindScore:
		if r.Score == nil {
			return a, fmt.Errorf("classifier: score question %q missing score", q.Name)
		}
		a.Score = *r.Score
		a.Probabilities = r.Probabilities
	}
	return a, nil
}

// canonicalChoice returns the question's original option matching value
// case-insensitively.
func canonicalChoice(options []string, value string) (string, bool) {
	for _, o := range options {
		if strings.EqualFold(o, value) {
			return o, true
		}
	}
	return "", false
}

// stripCodeFence removes a ```json ... ``` wrapper when present.
func stripCodeFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	t = strings.TrimPrefix(t, "```")
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[i+1:]
	}
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}

// SortedChoices returns candidate names in stable order for deterministic
// prompts.
func SortedChoices(choices []string) []string {
	out := append([]string(nil), choices...)
	sort.Strings(out)
	return out
}
