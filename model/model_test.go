package model

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestChatUsageAdd(t *testing.T) {
	u1 := ChatUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}
	u2 := ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	sum := u1.Add(u2)
	if sum.PromptTokens != 11 || sum.CompletionTokens != 22 || sum.TotalTokens != 33 {
		t.Fatalf("unexpected sum: %+v", sum)
	}
}

func TestChatOptions(t *testing.T) {
	opts := &ChatOptions{}
	WithMaxTokens(100)(opts)
	WithTemperature(0.5)(opts)
	WithTools([]ToolSpec{{Name: "tool1"}})(opts)
	tc := &ToolChoice{Mode: "auto", Function: "f1"}
	WithToolChoice(tc)(opts)

	if opts.MaxTokens != 100 {
		t.Fatalf("expected MaxTokens 100, got %d", opts.MaxTokens)
	}
	if opts.Temperature != 0.5 {
		t.Fatalf("expected Temperature 0.5, got %f", opts.Temperature)
	}
	if len(opts.Tools) != 1 || opts.Tools[0].Name != "tool1" {
		t.Fatalf("unexpected Tools: %+v", opts.Tools)
	}
	if opts.ToolChoice != tc {
		t.Fatal("expected ToolChoice to be set")
	}
}

func TestMultimodalRouter_ModelName(t *testing.T) {
	textModel := &mockModel{name: "text-model"}
	router := NewMultimodalRouter(textModel, nil)
	if router.ModelName() != "text-model" {
		t.Fatalf("expected text-model, got %s", router.ModelName())
	}

	router2 := NewMultimodalRouter(nil, &mockModel{name: "vision-model"})
	if router2.ModelName() != "vision-model" {
		t.Fatalf("expected vision-model, got %s", router2.ModelName())
	}

	router3 := NewMultimodalRouter(nil, nil)
	if router3.ModelName() != "multimodal-router" {
		t.Fatalf("expected multimodal-router, got %s", router3.ModelName())
	}
}

func TestMultimodalRouter_SelectModel_DataBlock(t *testing.T) {
	textModel := &mockModel{name: "text-model"}
	visionModel := &mockModel{name: "vision-model"}
	router := NewMultimodalRouter(textModel, visionModel)

	resp, err := router.Chat(nil, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).
			Content(message.NewDataBlock(message.TypeText, &message.Source{MediaType: "text/plain", Data: "x"})).
			Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "vision-model" {
		t.Fatalf("expected vision-model for DataBlock, got %s", resp.GetTextContent())
	}
}
