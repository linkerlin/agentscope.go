package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessageTool_Execute(t *testing.T) {
	mock := &mockFeishuServer{}
	srv := httptest.NewServer(mock.handler())
	defer srv.Close()
	c := New("f1", "app", "secret").WithBaseURL(srv.URL)

	tool := &SendMessageTool{Channel: c}
	resp, err := tool.Execute(context.Background(), map[string]any{"chat_id": "chat-1", "text": "hi from agent"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Sent") {
		t.Fatalf("resp = %q", resp.GetTextContent())
	}
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(mock.sent))
	}
	if mock.sent[0]["receive_id"] != "chat-1" {
		t.Fatalf("receive_id wrong: %+v", mock.sent[0])
	}
}

func TestSendMessageTool_MissingArgs(t *testing.T) {
	c := New("f1", "app", "secret")
	tool := &SendMessageTool{Channel: c}
	resp, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "required") {
		t.Fatalf("resp = %q", resp.GetTextContent())
	}
}

func TestListChatsTool_Execute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tok", "expire": 7200})
	})
	mux.HandleFunc("/im/v1/chats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"items": []map[string]any{
					{"chat_id": "c1", "name": "产品群"},
					{"chat_id": "c2", "name": "测试群"},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New("f1", "app", "secret").WithBaseURL(srv.URL)

	tool := &ListChatsTool{Channel: c}
	resp, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	out := resp.GetTextContent()
	if !strings.Contains(out, "产品群") || !strings.Contains(out, "c1") {
		t.Fatalf("chats output: %q", out)
	}
}

func TestToolsImplementToolInterface(t *testing.T) {
	var _ interface{ Name() string } = &SendMessageTool{}
	var _ interface{ Name() string } = &ListChatsTool{}
}
