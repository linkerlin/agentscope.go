package rag

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTikaClient_Parse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tika" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("extracted: " + string(body)))
	}))
	defer ts.Close()

	client := NewTikaClient(ts.URL)
	doc, err := client.Parse(context.Background(), strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if doc.Text != "extracted: hello world" {
		t.Fatalf("unexpected text: %s", doc.Text)
	}
}

func TestTikaClient_Parse_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer ts.Close()

	client := NewTikaClient(ts.URL)
	_, err := client.Parse(context.Background(), strings.NewReader("test"))
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}
