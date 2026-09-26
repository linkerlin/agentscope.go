// Package askuser provides the AskUser external tool: an agent-initiated
// question answered by the driving client (TUI, channel, gateway). It mirrors
// Python AgentScope's AskUser tool (PyV2 #2573).
package askuser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

const (
	minQuestions = 1
	maxQuestions = 4
	minOptions   = 2
	maxOptions   = 4
	maxHeader    = 12
)

// Option is one selectable answer.
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Preview     string `json:"preview,omitempty"`
}

// Question is a single question with 2-4 options.
type Question struct {
	Question    string   `json:"question"`
	Header      string   `json:"header"`
	Context     string   `json:"context,omitempty"`
	Options     []Option `json:"options"`
	MultiSelect bool     `json:"multi_select,omitempty"`
}

// Answer is the client's response to one question.
type Answer struct {
	Question string   `json:"question"`
	Selected []string `json:"selected"`
	Other    string   `json:"other,omitempty"`
}

// Metadata is the machine-readable answer payload transported in
// ExternalExecutionResult.Output as JSON.
type Metadata struct {
	Answers []Answer `json:"answers"`
}

// AskUserTool is an external tool: the agent suspends with a
// RequireExternalExecutionEvent and the driving client renders the questions,
// then supplies answers via ExternalExecutionResultEvent.
type AskUserTool struct{}

var (
	_ tool.Tool            = (*AskUserTool)(nil)
	_ tool.ExternalChecker = (*AskUserTool)(nil)
	_ tool.ReadOnlyChecker = (*AskUserTool)(nil)
)

// New creates an AskUser tool.
func New() *AskUserTool { return &AskUserTool{} }

// Name returns the tool name.
func (t *AskUserTool) Name() string { return "ask_user" }

// Description returns the tool description including the question constraints
// the model must honour.
func (t *AskUserTool) Description() string {
	return `Ask the user one to four structured questions when you need a decision, preference, or missing information before continuing.

Rules:
- Ask 1-4 questions per call; each needs a short header (max 12 characters).
- Each question needs 2-4 options. "Other" free-text input is always provided by the UI — never add your own "Other" option.
- Put the recommended option first and append " (Recommended)" to its label.
- Set multi_select only when several answers may be chosen together; preview is only supported for single-select questions.
- Use context to show any code or text the user needs in order to answer.
- The user's answers arrive as structured results; never guess them.`
}

// Spec returns the JSON schema for the tool parameters.
func (t *AskUserTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"questions"},
			"properties": map[string]any{
				"questions": map[string]any{
					"type":     "array",
					"minItems": minQuestions,
					"maxItems": maxQuestions,
					"items": map[string]any{
						"type":     "object",
						"required": []string{"question", "header", "options"},
						"properties": map[string]any{
							"question": map[string]any{"type": "string"},
							"header": map[string]any{
								"type":      "string",
								"maxLength": maxHeader,
							},
							"context":      map[string]any{"type": "string"},
							"multi_select": map[string]any{"type": "boolean"},
							"options": map[string]any{
								"type":     "array",
								"minItems": minOptions,
								"maxItems": maxOptions,
								"items": map[string]any{
									"type":     "object",
									"required": []string{"label", "description"},
									"properties": map[string]any{
										"label":       map[string]any{"type": "string"},
										"description": map[string]any{"type": "string"},
										"preview":     map[string]any{"type": "string"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Execute is never called for external tools; the driving client supplies the
// answers instead. Calling it directly is a programming error.
func (t *AskUserTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return nil, errors.New("ask_user is an external tool; the driving client must supply answers")
}

// IsExternalTool marks the tool for the external execution protocol.
func (t *AskUserTool) IsExternalTool() bool { return true }

// IsReadOnly reports that asking a question changes nothing.
func (t *AskUserTool) IsReadOnly() bool { return true }

// IsAutoApproved reports that asking the user never needs a permission prompt
// ("never asks to ask", PyV2 #2573).
func (t *AskUserTool) IsAutoApproved() bool { return true }

// MetadataSchema describes the shape of the answer payload clients send back.
func MetadataSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"answers"},
		"properties": map[string]any{
			"answers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"question", "selected"},
					"properties": map[string]any{
						"question": map[string]any{"type": "string"},
						"selected": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"other": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

// ValidateQuestions enforces the question constraints so malformed model
// output fails fast with an actionable message (PyV2 #2586).
func ValidateQuestions(questions []Question) error {
	if len(questions) < minQuestions || len(questions) > maxQuestions {
		return fmt.Errorf("ask_user: expected %d-%d questions, got %d", minQuestions, maxQuestions, len(questions))
	}
	seenQuestions := map[string]bool{}
	for i, q := range questions {
		text := strings.TrimSpace(q.Question)
		if text == "" {
			return fmt.Errorf("ask_user: question %d has empty text", i+1)
		}
		if seenQuestions[text] {
			return fmt.Errorf("ask_user: duplicate question %q", text)
		}
		seenQuestions[text] = true
		if utf8.RuneCountInString(q.Header) == 0 || utf8.RuneCountInString(q.Header) > maxHeader {
			return fmt.Errorf("ask_user: question %d header must be 1-%d characters, got %q", i+1, maxHeader, q.Header)
		}
		if len(q.Options) < minOptions || len(q.Options) > maxOptions {
			return fmt.Errorf("ask_user: question %d needs %d-%d options, got %d", i+1, minOptions, maxOptions, len(q.Options))
		}
		seenLabels := map[string]bool{}
		for j, o := range q.Options {
			label := strings.TrimSpace(o.Label)
			if label == "" {
				return fmt.Errorf("ask_user: question %d option %d has empty label", i+1, j+1)
			}
			if strings.EqualFold(label, "other") {
				return fmt.Errorf("ask_user: question %d must not define an \"Other\" option; the UI provides it", i+1)
			}
			if seenLabels[label] {
				return fmt.Errorf("ask_user: question %d has duplicate option %q", i+1, label)
			}
			seenLabels[label] = true
			if q.MultiSelect && o.Preview != "" {
				return fmt.Errorf("ask_user: question %d is multi-select and cannot use option previews", i+1)
			}
		}
	}
	return nil
}

// ParseMetadata decodes the answer payload clients place in
// ExternalExecutionResult.Output.
func ParseMetadata(output string) (*Metadata, error) {
	var meta Metadata
	if err := json.Unmarshal([]byte(output), &meta); err != nil {
		return nil, fmt.Errorf("ask_user: invalid answer payload: %w", err)
	}
	return &meta, nil
}

// FormatAnswersText renders answers as prose for the model to read.
func FormatAnswersText(meta *Metadata) string {
	if meta == nil || len(meta.Answers) == 0 {
		return "The user did not answer."
	}
	var sb strings.Builder
	for i, a := range meta.Answers {
		if i > 0 {
			sb.WriteString("\n")
		}
		if len(a.Selected) == 0 && a.Other == "" {
			fmt.Fprintf(&sb, "Q: %s\nA: (no answer)", a.Question)
			continue
		}
		fmt.Fprintf(&sb, "Q: %s\nA: %s", a.Question, strings.Join(a.Selected, ", "))
		if a.Other != "" {
			if len(a.Selected) > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString("Other: " + a.Other)
		}
	}
	return sb.String()
}
