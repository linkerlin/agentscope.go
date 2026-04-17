package rag

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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


func TestTikaClient_DefaultURL(t *testing.T) {
	c := NewTikaClient("")
	if c.BaseURL != "http://localhost:9998" {
		t.Fatalf("expected default URL, got %s", c.BaseURL)
	}
}

func TestTikaClient_ParseFile_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tika" && r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			w.Write([]byte("file: " + string(body)))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	f, err := os.CreateTemp("", "tika-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("hello tika")
	f.Close()

	client := NewTikaClient(ts.URL)
	doc, err := client.ParseFile(context.Background(), f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if doc.Text != "file: hello tika" {
		t.Fatalf("unexpected text: %s", doc.Text)
	}
}

func TestTikaClient_ParseFile_NotFound(t *testing.T) {
	client := NewTikaClient("http://localhost:9998")
	_, err := client.ParseFile(context.Background(), "/nonexistent/path.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
