package formatter

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// compile-time check that DashScopeFormatter implements Formatter
var _ Formatter = (*DashScopeFormatter)(nil)

func TestDashScopeFormatter_Basic(t *testing.T) {
	f := NewDashScopeFormatter()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	result, err := f.FormatMessages([]*message.Msg{msg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	tools, err := f.FormatTools([]model.ToolSpec{{Name: "t", Description: "d", Parameters: map[string]any{}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tools == nil {
		t.Fatal("expected non-nil tools")
	}
}
