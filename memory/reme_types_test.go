package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestSimpleTokenCounter(t *testing.T) {
	c := NewSimpleTokenCounter()
	n, err := c.Count("abcd")
	if err != nil || n != 1 {
		t.Fatalf("got %d err=%v", n, err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	}
	n2, err := c.CountMessages(msgs)
	if err != nil || n2 < 1 {
		t.Fatalf("messages tokens %d", n2)
	}
}

func TestMarkStore(t *testing.T) {
	s := NewMarkStore()
	s.Add("m1", MarkCompressed)
	if !s.Has("m1", MarkCompressed) {
		t.Fatal()
	}
	s.Add("m1", MarkCompressed)
	if len(s.Get("m1")) != 1 {
		t.Fatal()
	}
}
