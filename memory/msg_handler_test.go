package memory

import (
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestMarkStoreRoundTrip(t *testing.T) {
	var s *MarkStore
	if s.Get("x") != nil {
		t.Fatal("nil store")
	}
	s = NewMarkStore()
	s.Add("m1", MarkImportant)
	s.Add("m1", MarkImportant)
	if len(s.Get("m1")) != 1 {
		t.Fatal(s.Get("m1"))
	}
	if !s.Has("m1", MarkImportant) {
		t.Fatal()
	}
	s.Clear("m1")
	if len(s.Get("m1")) != 0 {
		t.Fatal()
	}
	s.Add("m2", MarkDeleted)
	m := s.ToMap()
	if len(m["m2"]) != 1 || m["m2"][0] != string(MarkDeleted) {
		t.Fatal(m)
	}
	loaded := LoadMarkStore(m)
	if !loaded.Has("m2", MarkDeleted) {
		t.Fatal()
	}
}

func TestFormatMessagesPlainToolUse(t *testing.T) {
	u := message.NewMsg().
		Role(message.RoleAssistant).
		Content(message.NewToolUseBlock("tid", "search", map[string]any{"q": "x"})).
		Build()
	out := FormatMessagesPlain([]*message.Msg{u})
	if !strings.Contains(out, "tool_use") || !strings.Contains(out, "tid") {
		t.Fatal(out)
	}
}

func TestFormatMessagesPlainNilSkip(t *testing.T) {
	u := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	out := FormatMessagesPlain([]*message.Msg{nil, u})
	if !strings.Contains(out, "hi") {
		t.Fatal(out)
	}
}
