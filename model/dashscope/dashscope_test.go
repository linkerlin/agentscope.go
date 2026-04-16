package dashscope

import (
	"testing"

	"github.com/linkerlin/agentscope.go/formatter"
)

func TestDashScopeBuilder_Formatter(t *testing.T) {
	f := formatter.NewDashScopeFormatter()
	m, err := Builder().
		APIKey("test-key").
		Formatter(f).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if m.ModelName() != "qwen-plus" {
		t.Fatalf("unexpected model name: %s", m.ModelName())
	}
}

func TestDashScopeBuilder_Default(t *testing.T) {
	m, err := Builder().
		APIKey("test-key").
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if m.ModelName() != "qwen-plus" {
		t.Fatalf("unexpected model name: %s", m.ModelName())
	}
}
