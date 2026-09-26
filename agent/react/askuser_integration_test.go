package react

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool/askuser"
)

// TestReActAgent_AskUserExternalExecution drives the full external-tool loop:
// the model calls ask_user, the agent suspends with a
// RequireExternalExecutionEvent, the client injects structured answers, and
// the turn completes with the model's follow-up.
func TestReActAgent_AskUserExternalExecution(t *testing.T) {
	askCall := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "ask_user", map[string]any{
			"questions": []any{
				map[string]any{
					"question": "Which database?",
					"header":   "Database",
					"options": []any{
						map[string]any{"label": "SQLite (Recommended)", "description": "embedded"},
						map[string]any{"label": "Postgres", "description": "server"},
					},
				},
			},
		}),
	).Build()
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("using sqlite").Build()

	m := &mockToolModel{name: "m", responses: []*message.Msg{askCall, finalMsg}}
	a, err := Builder().Name("t").Model(m).Tools(askuser.New()).Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	evCh, err := a.ReplyStream(ctx, message.NewMsg().Role(message.RoleUser).TextContent("set up a db").Build())
	if err != nil {
		t.Fatal(err)
	}

	var sawRequire bool
	var req *event.RequireExternalExecutionEvent
	for ev := range evCh {
		r, ok := ev.(*event.RequireExternalExecutionEvent)
		if !ok {
			continue
		}
		sawRequire = true
		req = r
		if len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "ask_user" {
			t.Fatalf("unexpected external tool calls: %+v", r.ToolCalls)
		}
		if err := a.InjectEvent(ctx, event.NewExternalExecutionResult(r.ReplyID(), r.ConfirmID, []event.ExternalExecutionResult{
			{
				ToolCallID: "call_1",
				Success:    true,
				Output:     `{"answers":[{"question":"Which database?","selected":["SQLite (Recommended)"]}]}`,
			},
		})); err != nil {
			t.Fatalf("inject failed: %v", err)
		}
	}
	if !sawRequire {
		t.Fatal("expected RequireExternalExecutionEvent")
	}
	if req == nil || req.ConfirmID == "" {
		t.Fatal("expected confirm id on the require event")
	}
	if m.calls < 2 {
		t.Fatalf("expected the loop to continue after answers, model calls=%d", m.calls)
	}
}

// TestReActAgent_AskUserAnswersParse verifies the injected payload round-trips
// through the helper the driving clients use.
func TestReActAgent_AskUserAnswersParse(t *testing.T) {
	meta, err := askuser.ParseMetadata(`{"answers":[{"question":"q","selected":["a"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(askuser.FormatAnswersText(meta), "a") {
		t.Fatalf("unexpected formatted answers")
	}
}
