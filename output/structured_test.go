package output

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type mockModel struct {
	responses []string
	idx       int
	err       error
}

func (m *mockModel) ModelName() string { return "mock" }

func (m *mockModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	r := m.responses[m.idx]
	m.idx++
	return message.NewMsg().Role(message.RoleAssistant).TextContent(r).Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func TestParseJSONFromAssistant(t *testing.T) {
	var out struct {
		X int `json:"x"`
	}
	err := ParseJSONFromAssistant("here {\"x\": 2}", &out)
	if err != nil || out.X != 2 {
		t.Fatal(err, out.X)
	}
}

func TestParseJSONFromAssistant_MarkdownFences(t *testing.T) {
	var out struct {
		X int `json:"x"`
	}
	err := ParseJSONFromAssistant("```json\n{\"x\":1}\n```", &out)
	if err != nil || out.X != 1 {
		t.Fatalf("expected x=1, got err=%v, x=%d", err, out.X)
	}
}

func TestParseJSONFromAssistant_NestedBraces(t *testing.T) {
	var out struct {
		A struct {
			B int `json:"b"`
		} `json:"a"`
	}
	err := ParseJSONFromAssistant("prefix {\"a\":{\"b\":1}} suffix", &out)
	if err != nil || out.A.B != 1 {
		t.Fatalf("expected a.b=1, got err=%v, b=%d", err, out.A.B)
	}
}

func TestStructuredRunner_Run_Success(t *testing.T) {
	m := &mockModel{responses: []string{`{"x":42}`}}
	runner := &StructuredRunner{Model: m}
	schema := &JSONSchema{Type: "object"}
	var out struct {
		X int `json:"x"`
	}
	err := runner.Run(context.Background(), "test", schema, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.X != 42 {
		t.Fatalf("expected x=42, got %d", out.X)
	}
}

func TestStructuredRunner_Run_NilModel(t *testing.T) {
	runner := &StructuredRunner{Model: nil}
	schema := &JSONSchema{Type: "object"}
	var out struct{}
	err := runner.Run(context.Background(), "test", schema, &out)
	if err == nil || err.Error() != "output: nil model" {
		t.Fatalf("expected nil model error, got %v", err)
	}
}

func TestStructuredRunner_Run_NilSchema(t *testing.T) {
	m := &mockModel{responses: []string{`{}`}}
	runner := &StructuredRunner{Model: m}
	var out struct{}
	err := runner.Run(context.Background(), "test", nil, &out)
	if err == nil || err.Error() != "output: nil schema" {
		t.Fatalf("expected nil schema error, got %v", err)
	}
}

func TestStructuredRunner_Run_MaxRetries_SelfCorrection(t *testing.T) {
	m := &mockModel{responses: []string{`{"x":1`, `{"x":99}`}}
	runner := &StructuredRunner{Model: m, MaxRetries: 2}
	schema := &JSONSchema{Type: "object"}
	var out struct {
		X int `json:"x"`
	}
	err := runner.Run(context.Background(), "test", schema, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.X != 99 {
		t.Fatalf("expected x=99 after correction, got %d", out.X)
	}
	if m.idx != 2 {
		t.Fatalf("expected 2 model calls, got %d", m.idx)
	}
}

func TestSelfCorrectingParser_ParseWithCorrection_MaxRetriesExhausted(t *testing.T) {
	m := &mockModel{responses: []string{`bad`, `worse`}}
	parser := &SelfCorrectingParser{Model: m, MaxRetries: 3}
	schema := &JSONSchema{Type: "object"}
	var out struct{}
	err := parser.ParseWithCorrection(context.Background(), "initial bad", schema, &out)
	if err == nil {
		t.Fatal("expected error after max retries exhausted")
	}
	if m.idx != 2 {
		t.Fatalf("expected 2 model calls, got %d", m.idx)
	}
}

func TestSelfCorrectingParser_ParseWithCorrection_ModelError(t *testing.T) {
	modelErr := errors.New("model failed")
	m := &mockModel{responses: []string{`{"x":1`}, err: modelErr}
	parser := &SelfCorrectingParser{Model: m, MaxRetries: 2}
	schema := &JSONSchema{Type: "object"}
	var out struct{}
	err := parser.ParseWithCorrection(context.Background(), `{"x":1`, schema, &out)
	if !errors.Is(err, modelErr) {
		t.Fatalf("expected model error %v, got %v", modelErr, err)
	}
}
