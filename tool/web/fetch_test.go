package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/tool"
)

func TestFetchTool_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello world"))
	}))
	defer ts.Close()

	ft := NewFetchTool(5 * time.Second)
	resp, err := ft.Execute(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "hello world") {
		t.Fatalf("expected 'hello world' in response, got: %s", text)
	}
	if !strings.Contains(text, "HTTP 200") {
		t.Fatalf("expected HTTP status in response, got: %s", text)
	}
}

func TestFetchTool_MissingURL(t *testing.T) {
	ft := NewFetchTool(5 * time.Second)
	resp, err := ft.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "WebFetchError") {
		t.Fatalf("expected error, got: %s", resp.GetTextContent())
	}
}

func TestFetchTool_InvalidScheme(t *testing.T) {
	ft := NewFetchTool(5 * time.Second)
	resp, err := ft.Execute(context.Background(), map[string]any{"url": "ftp://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "WebFetchError") {
		t.Fatalf("expected error for invalid scheme, got: %s", resp.GetTextContent())
	}
}

func TestFetchTool_MaxLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("a very long response body"))
	}))
	defer ts.Close()

	ft := NewFetchTool(5 * time.Second)
	resp, err := ft.Execute(context.Background(), map[string]any{"url": ts.URL, "max_length": float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	// After the HTTP header, we should get only 5 chars.
	bodyPart := strings.SplitN(text, "\n\n", 2)
	if len(bodyPart) != 2 || len(bodyPart[1]) > 5 {
		t.Fatalf("expected body truncated to 5 chars, got: %s", text)
	}
}

func TestFetchTool_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer ts.Close()

	ft := NewFetchTool(5 * time.Second)
	resp, err := ft.Execute(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "HTTP 404") {
		t.Fatalf("expected 404 status, got: %s", resp.GetTextContent())
	}
}

func TestFetchTool_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()

	ft := NewFetchTool(50 * time.Millisecond)
	resp, err := ft.Execute(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "WebFetchError") {
		t.Fatalf("expected timeout error, got: %s", resp.GetTextContent())
	}
}

func TestFetchTool_Spec(t *testing.T) {
	ft := NewFetchTool(10 * time.Second)
	spec := ft.Spec()
	if spec.Name != "web_fetch" {
		t.Fatalf("expected web_fetch, got %s", spec.Name)
	}
	if spec.Description == "" {
		t.Fatal("expected non-empty description")
	}
	params := spec.Parameters
	if params == nil {
		t.Fatal("expected parameters")
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["url"]; !ok {
		t.Fatal("expected url property")
	}
}

func TestFetchTool_InterfaceCheck(t *testing.T) {
	var _ tool.Tool = (*FetchTool)(nil)
}

func TestFetchTool_IsReadOnly(t *testing.T) {
	ft := NewFetchTool(10 * time.Second)
	if !ft.IsReadOnly() {
		t.Fatal("expected read-only tool")
	}
}

func TestFetchTool_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Write([]byte("slow"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ft := NewFetchTool(5 * time.Second)
	resp, err := ft.Execute(ctx, map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "WebFetchError") {
		t.Fatalf("expected context cancellation error, got: %s", resp.GetTextContent())
	}
}
