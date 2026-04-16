package gemini

import (
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestBuildRequestBody(t *testing.T) {
	m, err := NewBuilder().APIKey("test-key").ModelName("gemini-pro").Build()
	if err != nil {
		t.Fatal(err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("system prompt").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	}
	body, err := m.buildRequestBody(msgs, false, model.WithTemperature(0.5), model.WithMaxTokens(100))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `"contents"`) {
		t.Errorf("expected contents in body: %s", s)
	}
	if !strings.Contains(s, `"systemInstruction"`) {
		t.Errorf("expected systemInstruction in body: %s", s)
	}
	if !strings.Contains(s, `"temperature"`) {
		t.Errorf("expected temperature in body: %s", s)
	}
	if !strings.Contains(s, `"maxOutputTokens"`) {
		t.Errorf("expected maxOutputTokens in body: %s", s)
	}
}

func TestBuildRequestBodyWithTools(t *testing.T) {
	m, err := NewBuilder().APIKey("test-key").Build()
	if err != nil {
		t.Fatal(err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}
	tools := []model.ToolSpec{
		{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}},
	}
	body, err := m.buildRequestBody(msgs, false, model.WithTools(tools), model.WithToolChoice(&model.ToolChoice{Mode: "any", Function: "echo"}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `"functionDeclarations"`) {
		t.Errorf("expected functionDeclarations in body: %s", s)
	}
	if !strings.Contains(s, `"functionCallingConfig"`) {
		t.Errorf("expected functionCallingConfig in body: %s", s)
	}
	if !strings.Contains(s, `"allowedFunctionNames"`) {
		t.Errorf("expected allowedFunctionNames in body: %s", s)
	}
}

func TestParseResponse(t *testing.T) {
	m, _ := NewBuilder().APIKey("test-key").Build()
	resp := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role":  "model",
					"parts": []any{map[string]any{"text": "world"}},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     1.0,
			"candidatesTokenCount": 2.0,
			"totalTokenCount":      3.0,
		},
	}
	msg, err := m.fmt.ParseResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "world" {
		t.Errorf("expected 'world', got %q", msg.GetTextContent())
	}
	usage, _ := msg.Metadata["usage"].(model.ChatUsage)
	if usage.TotalTokens != 3 {
		t.Errorf("expected total tokens 3, got %d", usage.TotalTokens)
	}
}
