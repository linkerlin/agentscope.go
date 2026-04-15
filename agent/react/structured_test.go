package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/output"
)

type structuredMockModel struct {
	calls int
}

func (m *structuredMockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.calls++
	// First call returns invalid JSON, second call returns valid JSON (thanks to self-correction)
	if m.calls == 1 {
		return message.NewMsg().Role(message.RoleAssistant).TextContent(`{"name": "Alice"`).Build(), nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(`{"name": "Alice", "age": 30}`).Build(), nil
}

func (m *structuredMockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func (m *structuredMockModel) ModelName() string { return "mock" }

func TestReActAgent_CallStructured_SelfCorrecting(t *testing.T) {
	m := &structuredMockModel{}
	agent, err := Builder().
		Name("Test").
		Model(m).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	schema := &output.JSONSchema{
		Type: "object",
		Properties: map[string]*output.SchemaProp{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name", "age"},
	}

	var result struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err = agent.CallStructured(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("give me json").Build(), schema, &result)
	if err != nil {
		t.Fatalf("CallStructured failed: %v", err)
	}

	if result.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", result.Name)
	}
	if result.Age != 30 {
		t.Fatalf("expected age 30, got %d", result.Age)
	}
	if m.calls < 2 {
		t.Fatalf("expected at least 2 model calls due to self-correction, got %d", m.calls)
	}
}
