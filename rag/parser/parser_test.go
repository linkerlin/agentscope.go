package parser

import (
	"context"
	"strings"
	"testing"
)

func TestTextParser_DefaultMediaTypes(t *testing.T) {
	p := NewTextParser()
	types := p.SupportedMediaTypes()
	if len(types) == 0 {
		t.Fatal("expected default media types")
	}
	found := false
	for _, mt := range types {
		if mt == "text/plain" {
			found = true
		}
	}
	if !found {
		t.Fatal("text/plain missing from defaults")
	}
}

func TestTextParser_Parse(t *testing.T) {
	p := NewTextParser()
	secs, err := p.Parse(context.Background(), strings.NewReader("hello world"), "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if secs[0].Text() != "hello world" {
		t.Fatalf("got %q", secs[0].Text())
	}
	if secs[0].Source != "a.txt" {
		t.Fatalf("source not carried: %q", secs[0].Source)
	}
}

func TestTextParser_CustomMediaTypes(t *testing.T) {
	p := &TextParser{MediaTypes: []string{"text/x-log"}}
	if got := p.SupportedMediaTypes(); len(got) != 1 || got[0] != "text/x-log" {
		t.Fatalf("custom types ignored: %v", got)
	}
}

func TestRegistry_RouteAndParse(t *testing.T) {
	reg := NewRegistry(NewTextParser())
	if _, ok := reg.Get("text/plain"); !ok {
		t.Fatal("text/plain not registered")
	}
	if _, ok := reg.Get("application/pdf"); ok {
		t.Fatal("pdf should not be registered")
	}

	secs, err := reg.Parse(context.Background(), "text/plain", "in.txt", strings.NewReader("body"))
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 1 || secs[0].Text() != "body" {
		t.Fatalf("unexpected sections: %+v", secs)
	}
}

func TestRegistry_UnknownType(t *testing.T) {
	reg := NewRegistry(NewTextParser())
	_, err := reg.Parse(context.Background(), "application/pdf", "x.pdf", strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for unregistered type")
	}
}

func TestRegistry_RegisterReplaces(t *testing.T) {
	reg := NewRegistry()
	p1 := NewTextParser()
	p2 := &TextParser{MediaTypes: []string{"text/plain"}}
	reg.Register(p1)
	reg.Register(p2)
	got, _ := reg.Get("text/plain")
	if got != p2 {
		t.Fatal("Register should replace existing parser")
	}
}

// compile-time assertion that TextParser satisfies Parser.
var _ Parser = (*TextParser)(nil)
