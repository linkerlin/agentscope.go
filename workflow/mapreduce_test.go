package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestMapReduce_Call_Success(t *testing.T) {
	split := func(m *message.Msg) []string {
		return strings.Split(m.GetTextContent(), ",")
	}
	mapper := &mockAgent{name: "mapper", resp: "mapped"}
	reducer := &mockAgent{name: "reducer", resp: "done"}

	mr := NewMapReduce("mr", split, mapper, reducer, 2)
	resp, err := mr.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("a,b,c").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "done" {
		t.Fatalf("expected 'done', got %q", resp.GetTextContent())
	}
}

func TestMapReduce_Call_MapperError(t *testing.T) {
	split := func(m *message.Msg) []string {
		return []string{"ok", "bad"}
	}
	mapper := &mockAgent{name: "mapper", err: errors.New("boom")}
	reducer := &mockAgent{name: "reducer", resp: "final"}

	mr := NewMapReduce("mr", split, mapper, reducer, 2)
	_, err := mr.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reducer.lastInput, "error: boom") {
		t.Fatalf("expected error text in reducer input, got %q", reducer.lastInput)
	}
}

func TestMapReduce_Call_NilDeps(t *testing.T) {
	mr := NewMapReduce("mr", nil, nil, nil, 2)
	_, err := mr.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err == nil {
		t.Fatal("expected error for nil deps")
	}
}

func TestMapReduce_Call_EmptyChunks(t *testing.T) {
	split := func(m *message.Msg) []string { return []string{} }
	mr := NewMapReduce("mr", split, &mockAgent{name: "m"}, &mockAgent{name: "r"}, 2)
	_, err := mr.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err == nil {
		t.Fatal("expected error for empty chunks")
	}
}

func TestMapReduce_CallStream_NotSupported(t *testing.T) {
	mr := NewMapReduce("mr", nil, &mockAgent{name: "m"}, &mockAgent{name: "r"}, 2)
	_, err := mr.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}
