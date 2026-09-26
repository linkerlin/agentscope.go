package console

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/tool/askuser"
)

func askUserEvent() *event.RequireExternalExecutionEvent {
	return event.NewRequireExternalExecution("r1", "cfm-1", []event.ToolCallSummary{{
		ID:   "call_1",
		Name: "ask_user",
		Input: map[string]any{
			"questions": []any{
				map[string]any{
					"question": "Which database?",
					"header":   "Database",
					"options": []any{
						map[string]any{"label": "SQLite (Recommended)", "description": "embedded"},
						map[string]any{"label": "Postgres", "description": "server"},
					},
				},
				map[string]any{
					"question":     "Which features?",
					"header":       "Features",
					"multi_select": true,
					"options": []any{
						map[string]any{"label": "Auth", "description": "login"},
						map[string]any{"label": "API", "description": "rest"},
					},
				},
			},
		},
	}})
}

func TestConsole_AskUserFlow(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{askUserEvent()}}
	m := newTestModel(fa)
	m.startReply("setup")
	for m.phase == phaseRunning {
		m.handleEvent(<-fa.stream)
	}
	if m.phase != phaseAskUser {
		t.Fatalf("expected ask_user phase, got %v", m.phase)
	}
	if len(m.askQuestions) != 2 {
		t.Fatalf("expected 2 parsed questions, got %d", len(m.askQuestions))
	}
	if out := joined(m); !strings.Contains(out, "[Database] Which database?") {
		t.Fatalf("expected question rendered:\n%s", out)
	}

	// Answer the single-select question with option 2.
	m.input.SetValue("2")
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseAskUser {
		t.Fatalf("expected to stay in ask_user for question 2, got %v", m.phase)
	}

	// Answer the multi-select question with "1,2".
	m.input.SetValue("1,2")
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if len(fa.injected) != 1 {
		t.Fatalf("expected one injected result, got %d", len(fa.injected))
	}
	ext, ok := fa.injected[0].(*event.ExternalExecutionResultEvent)
	if !ok {
		t.Fatalf("unexpected injected event %T", fa.injected[0])
	}
	if len(ext.Results) != 1 || !ext.Results[0].Success {
		t.Fatalf("unexpected results: %+v", ext.Results)
	}
	meta, err := askuser.ParseMetadata(ext.Results[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Answers) != 2 {
		t.Fatalf("expected 2 answers, got %+v", meta)
	}
	if meta.Answers[0].Selected[0] != "Postgres" {
		t.Fatalf("unexpected single-select answer: %+v", meta.Answers[0])
	}
	if len(meta.Answers[1].Selected) != 2 || meta.Answers[1].Selected[0] != "Auth" || meta.Answers[1].Selected[1] != "API" {
		t.Fatalf("unexpected multi-select answer: %+v", meta.Answers[1])
	}
}

func TestConsole_AskUserFreeText(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{askUserEvent()}}
	m := newTestModel(fa)
	m.startReply("setup")
	for m.phase == phaseRunning {
		m.handleEvent(<-fa.stream)
	}

	m.input.SetValue("mysql")
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	// Skip the second question with a free-text answer too.
	m.input.SetValue("cache")
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	ext, _ := fa.injected[0].(*event.ExternalExecutionResultEvent)
	meta, err := askuser.ParseMetadata(ext.Results[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Answers[0].Other != "mysql" || meta.Answers[0].Selected != nil {
		t.Fatalf("unexpected free-text answer: %+v", meta.Answers[0])
	}
}

func TestConsole_AskUserCtrlCCancels(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{askUserEvent()}}
	m := newTestModel(fa)
	m.startReply("setup")
	for m.phase == phaseRunning {
		m.handleEvent(<-fa.stream)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if len(fa.injected) != 1 {
		t.Fatalf("expected one injected result, got %d", len(fa.injected))
	}
	ext, _ := fa.injected[0].(*event.ExternalExecutionResultEvent)
	if ext.Results[0].Success {
		t.Fatalf("expected cancelled result, got %+v", ext.Results[0])
	}
	if ext.Results[0].Error != "user cancelled" {
		t.Fatalf("unexpected cancel reason: %q", ext.Results[0].Error)
	}
}

func TestInterpretAnswer(t *testing.T) {
	q := askuser.Question{
		Question: "pick",
		Options: []askuser.Option{
			{Label: "A"},
			{Label: "B"},
			{Label: "C"},
		},
	}
	selected, other := interpretAnswer(q, "2")
	if len(selected) != 1 || selected[0] != "B" || other != "" {
		t.Fatalf("unexpected: %v %q", selected, other)
	}

	selected, other = interpretAnswer(q, "free text")
	if len(selected) != 0 || other != "free text" {
		t.Fatalf("unexpected: %v %q", selected, other)
	}

	multi := q
	multi.MultiSelect = true
	selected, _ = interpretAnswer(multi, "1, 3")
	if len(selected) != 2 || selected[0] != "A" || selected[1] != "C" {
		t.Fatalf("unexpected multi: %v", selected)
	}

	// Out-of-range numbers count as free text.
	selected, other = interpretAnswer(q, "9")
	if len(selected) != 0 || other != "9" {
		t.Fatalf("unexpected: %v %q", selected, other)
	}
}

func TestParseAskQuestions(t *testing.T) {
	qs := parseAskQuestions(map[string]any{
		"questions": []any{
			map[string]any{
				"question": "q",
				"header":   "h",
				"options": []any{
					map[string]any{"label": "a", "description": "d"},
					map[string]any{"label": "b", "description": "e"},
				},
			},
		},
	})
	if len(qs) != 1 || qs[0].Header != "h" || len(qs[0].Options) != 2 {
		t.Fatalf("unexpected parse: %+v", qs)
	}
	if got := parseAskQuestions(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %+v", got)
	}
	if got := parseAskQuestions(map[string]any{"questions": "not-an-array"}); got != nil {
		t.Fatalf("expected nil for malformed input, got %+v", got)
	}
}
