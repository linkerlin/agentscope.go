package sop

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/output"
)

// scriptAgent is a scripted AgentLike for engine tests.
type scriptAgent struct {
	mu           sync.Mutex
	texts        []string
	handovers    []string
	verdicts     []verdictResult
	parkRounds   map[int]bool
	calls        int
	resumeCh     chan event.AgentEvent
	injected     []event.AgentEvent
	instructions []string
}

func newScriptAgent() *scriptAgent {
	return &scriptAgent{resumeCh: make(chan event.AgentEvent, 4)}
}

func (a *scriptAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	a.mu.Lock()
	call := a.calls
	a.calls++
	a.mu.Unlock()
	if msg != nil {
		a.mu.Lock()
		a.instructions = append(a.instructions, msg.GetTextContent())
		a.mu.Unlock()
	}
	ch := make(chan event.AgentEvent, 16)
	go func() {
		if a.parkRounds[call] {
			ch <- event.NewRequireUserConfirm("r", "cfm", nil)
			select {
			case <-a.resumeCh:
			case <-ctx.Done():
				close(ch)
				return
			}
		}
		text := ""
		a.mu.Lock()
		if len(a.texts) > 0 {
			text = a.texts[call%len(a.texts)]
		}
		a.mu.Unlock()
		if text != "" {
			ch <- event.NewTextBlockDelta("r", 0, text)
		}
		ch <- event.NewReplyEnd("r", "script")
		close(ch)
	}()
	return ch, nil
}

func (a *scriptAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	a.mu.Lock()
	a.injected = append(a.injected, ev)
	a.mu.Unlock()
	select {
	case a.resumeCh <- ev:
	default:
	}
	return nil
}

func (a *scriptAgent) CallStructured(ctx context.Context, user *message.Msg, schema *output.JSONSchema, target any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch v := target.(type) {
	case *handoverResult:
		if len(a.handovers) == 0 {
			v.Handover = "handover"
			return nil
		}
		v.Handover = a.handovers[0]
		a.handovers = a.handovers[1:]
	case *verdictResult:
		if len(a.verdicts) == 0 {
			v.Passed = true
			return nil
		}
		*v = a.verdicts[0]
		a.verdicts = a.verdicts[1:]
	}
	return nil
}

func drainEvents(ch <-chan event.AgentEvent) []event.AgentEvent {
	var out []event.AgentEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func TestEngine_HappyPath(t *testing.T) {
	exec := newScriptAgent()
	exec.texts = []string{"did step one", "did step two"}
	exec.handovers = []string{"one done", "two done"}
	ver := newScriptAgent()
	ver.verdicts = []verdictResult{{Passed: true}, {Passed: true}}

	sop := &SOP{Name: "demo", Steps: []Step{
		{Subject: "one", Description: "first", Executor: exec, Verifier: ver},
		{Subject: "two", Description: "second", Executor: exec, Verifier: ver},
	}}
	eng, err := NewEngine(sop, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := eng.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(ch)

	st := eng.State()
	if st.Phase() != PhaseCompleted {
		t.Fatalf("expected completed run, got %s", st.Phase())
	}
	if st.Steps[0].Submission != "one done" || st.Steps[1].Submission != "two done" {
		t.Fatalf("unexpected submissions: %+v", st.Steps)
	}
	// Step 2 must receive step 1's handover, not the raw dialog.
	if !strings.Contains(st.Steps[1].Given, "one done") || !strings.Contains(st.Steps[1].Given, "<handover") {
		t.Fatalf("handover ledger missing in step 2 input: %q", st.Steps[1].Given)
	}
	if st.Steps[0].Given != "go" {
		t.Fatalf("step 0 must receive run inputs, got %q", st.Steps[0].Given)
	}
}

func TestEngine_RejectThenPass(t *testing.T) {
	exec := newScriptAgent()
	exec.texts = []string{"try one", "try two"}
	exec.handovers = []string{"bad", "good"}
	ver := newScriptAgent()
	ver.verdicts = []verdictResult{
		{Passed: false, Message: "missing tests"},
		{Passed: true},
	}
	sop := &SOP{Name: "demo", Steps: []Step{
		{Subject: "ship", Description: "ship it", Executor: exec, Verifier: ver},
	}}
	eng, err := NewEngine(sop, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := eng.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	drainEvents(ch)

	st := eng.State()
	if st.Phase() != PhaseCompleted {
		t.Fatalf("expected completed, got %s", st.Phase())
	}
	if len(st.Steps[0].Verifications) != 2 {
		t.Fatalf("expected 2 verifications, got %+v", st.Steps[0].Verifications)
	}
	if st.Steps[0].Submission != "good" {
		t.Fatalf("unexpected final submission: %q", st.Steps[0].Submission)
	}
}

func TestEngine_MaxAttemptsFailed(t *testing.T) {
	exec := newScriptAgent()
	exec.handovers = []string{"bad", "bad", "bad"}
	ver := newScriptAgent()
	ver.verdicts = []verdictResult{
		{Passed: false, Message: "no"}, {Passed: false, Message: "no"}, {Passed: false, Message: "no"},
	}
	sop := &SOP{Name: "demo", Steps: []Step{
		{Subject: "ship", Executor: exec, Verifier: ver, MaxAttempts: 3},
	}}
	eng, err := NewEngine(sop, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := eng.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	drainEvents(ch)

	if eng.State().Phase() != PhaseFailed {
		t.Fatalf("expected failed run, got %s", eng.State().Phase())
	}
}

func TestEngine_NoVerifierPasses(t *testing.T) {
	exec := newScriptAgent()
	exec.handovers = []string{"done"}
	sop := &SOP{Name: "demo", Steps: []Step{
		{Subject: "solo", Executor: exec},
	}}
	eng, err := NewEngine(sop, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := eng.ReplyStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	drainEvents(ch)
	if eng.State().Phase() != PhaseCompleted {
		t.Fatalf("expected completed, got %s", eng.State().Phase())
	}
}

func TestEngine_ParkAndResume(t *testing.T) {
	exec := newScriptAgent()
	exec.texts = []string{"resumed work"}
	exec.handovers = []string{"done after resume"}
	exec.parkRounds = map[int]bool{0: true}
	ver := newScriptAgent()
	ver.verdicts = []verdictResult{{Passed: true}}

	sop := &SOP{Name: "demo", Steps: []Step{
		{Subject: "one", Executor: exec, Verifier: ver},
	}}
	eng, err := NewEngine(sop, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ch, _ := eng.ReplyStream(ctx, message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	events := drainEvents(ch)

	sawRequire := false
	for _, ev := range events {
		if _, ok := ev.(*event.RequireUserConfirmEvent); ok {
			sawRequire = true
		}
	}
	if !sawRequire {
		t.Fatal("expected the park event in the first stream")
	}
	if !eng.Suspended() {
		t.Fatal("expected engine to be suspended after park")
	}
	if eng.State().Steps[0].Phase != PhaseAwaiting {
		t.Fatalf("expected awaiting step, got %s", eng.State().Steps[0].Phase)
	}

	// Resume: inject the answer, then stream again.
	if err := eng.InjectEvent(ctx, event.NewUserConfirmResult("r", "cfm", nil)); err != nil {
		t.Fatal(err)
	}
	ch2, _ := eng.ReplyStream(ctx, message.NewMsg().Role(message.RoleUser).TextContent("confirmed").Build())
	drainEvents(ch2)

	if eng.Suspended() {
		t.Fatal("expected suspension to clear after resume")
	}
	if eng.State().Phase() != PhaseCompleted {
		t.Fatalf("expected completed after resume, got %s", eng.State().Phase())
	}
	if len(exec.injected) != 1 {
		t.Fatalf("expected the answer forwarded once, got %d", len(exec.injected))
	}
}

func TestEngine_Validation(t *testing.T) {
	exec := newScriptAgent()
	if _, err := NewEngine(nil, nil); err == nil {
		t.Fatal("expected error for nil SOP")
	}
	if _, err := NewEngine(&SOP{Name: "empty"}, nil); err == nil {
		t.Fatal("expected error for SOP without steps")
	}
	if _, err := NewEngine(&SOP{Name: "x", Steps: []Step{{Subject: ""}}}, nil); err == nil {
		t.Fatal("expected error for step without subject")
	}
	if _, err := NewEngine(&SOP{Name: "x", Steps: []Step{{Subject: "s"}}}, nil); err == nil {
		t.Fatal("expected error for step without executor")
	}
	sop := &SOP{Name: "x", Steps: []Step{{Subject: "s", Executor: exec}}}
	st := NewRunState(sop, "go")
	st.Steps = append(st.Steps, StepState{})
	if _, err := NewEngine(sop, st); err == nil {
		t.Fatal("expected error for state/SOP step count mismatch")
	}
}

func TestRunState_Phase(t *testing.T) {
	cases := []struct {
		phases []Phase
		want   Phase
	}{
		{[]Phase{PhasePending, PhasePending}, PhasePending},
		{[]Phase{PhaseCompleted, PhaseCompleted}, PhaseCompleted},
		{[]Phase{PhaseCompleted, PhaseFailed}, PhaseFailed},
		{[]Phase{PhaseCompleted, PhaseAwaiting}, PhaseAwaiting},
		{[]Phase{PhaseCompleted, PhaseRunning}, PhaseRunning},
		{[]Phase{PhaseCompleted, PhasePending}, PhaseRunning},
	}
	for _, c := range cases {
		states := make([]StepState, len(c.phases))
		for i, p := range c.phases {
			states[i].Phase = p
		}
		if got := (&RunState{Steps: states}).Phase(); got != c.want {
			t.Fatalf("phases %v: expected %s, got %s", c.phases, c.want, got)
		}
	}
	if got := (*RunState)(nil).Phase(); got != PhasePending {
		t.Fatalf("expected pending for nil state, got %s", got)
	}
}

func TestRunState_JSONRoundTrip(t *testing.T) {
	sop := &SOP{Name: "x", Steps: []Step{{Subject: "s", Executor: newScriptAgent()}}}
	st := NewRunState(sop, "go")
	st.Steps[0].Phase = PhaseCompleted
	st.Steps[0].Submission = "done"
	st.Steps[0].HasSubmission = true
	st.Steps[0].Verifications = []VerificationResult{{Passed: true, Message: "ok"}}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var back RunState
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Phase() != PhaseCompleted || back.Steps[0].Submission != "done" {
		t.Fatalf("round trip lost state: %+v", back)
	}
}

func TestStepEvent_ImplementsAgentEvent(t *testing.T) {
	ev := &StepEvent{Name: "sop_step_started"}
	var _ event.AgentEvent = ev
	if ev.EventType() != "sop_step_started" {
		t.Fatalf("unexpected event type: %s", ev.EventType())
	}
}
