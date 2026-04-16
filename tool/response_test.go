package tool

import (
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestNewTextResponse_String(t *testing.T) {
	r := NewTextResponse("hello")
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(r.Content))
	}
	if r.GetTextContent() != "hello" {
		t.Fatalf("expected hello, got %s", r.GetTextContent())
	}
	if !r.IsLast {
		t.Fatal("expected IsLast=true")
	}
}

func TestNewTextResponse_Number(t *testing.T) {
	r := NewTextResponse(42)
	if r.GetTextContent() != "42" {
		t.Fatalf("expected 42, got %s", r.GetTextContent())
	}
}

func TestNewTextResponse_Map(t *testing.T) {
	r := NewTextResponse(map[string]any{"k": "v"})
	if r.GetTextContent() == "" {
		t.Fatal("expected non-empty JSON text")
	}
}

func TestNewTextResponse_PassthroughResponse(t *testing.T) {
	orig := &Response{Content: []message.ContentBlock{message.NewTextBlock("passthrough")}}
	r := NewTextResponse(orig)
	if r != orig {
		t.Fatal("expected same pointer for *Response input")
	}
}

func TestNewErrorResponse(t *testing.T) {
	r := NewErrorResponse(errors.New("boom"))
	if r.GetTextContent() != "boom" {
		t.Fatalf("expected boom, got %s", r.GetTextContent())
	}
}

func TestResponse_GetTextContent_Multimedia(t *testing.T) {
	r := &Response{
		Content: []message.ContentBlock{
			message.NewTextBlock("a"),
			message.NewImageBlock("http://x", "", "image/png"),
			message.NewTextBlock("b"),
		},
	}
	if r.GetTextContent() != "ab" {
		t.Fatalf("expected ab, got %s", r.GetTextContent())
	}
}

func TestResponse_GetTextContent_Nil(t *testing.T) {
	var r *Response
	if r.GetTextContent() != "" {
		t.Fatal("expected empty string for nil response")
	}
}
