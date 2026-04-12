package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestSplitRespectsToolPairs(t *testing.T) {
	u := message.NewMsg().Role(message.RoleUser).TextContent("u").Build()
	asst := message.NewMsg().Role(message.RoleAssistant).Content(message.NewToolUseBlock("t1", "echo", map[string]any{})).Build()
	tr := message.NewMsg().Role(message.RoleTool).Content(message.NewToolResultBlock("t1", []message.ContentBlock{message.NewTextBlock("ok")}, false)).Build()
	msgs := []*message.Msg{u, asst, tr}
	if splitRespectsToolPairs(msgs, 2) {
		t.Fatal("split at 2 leaves pending tool_use in compact")
	}
	if !splitRespectsToolPairs(msgs, 3) {
		t.Fatal("split at 3 should be valid")
	}
}

func TestCheckContextUnderThreshold(t *testing.T) {
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}
	cc, err := CheckContext(context.Background(), msgs, 1000, 100, NewSimpleTokenCounter())
	if err != nil || len(cc.MessagesToCompact) != 0 {
		t.Fatalf("%+v err=%v", cc, err)
	}
}
