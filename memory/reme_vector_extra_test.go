package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestMessagesToMemoryNodes(t *testing.T) {
	u := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	msgs := []*message.Msg{nil, u}
	out := MessagesToMemoryNodes(msgs, MemoryTypePersonal, "alice")
	if len(out) != 1 || out[0].MemoryTarget != "alice" {
		t.Fatal(len(out))
	}
}
