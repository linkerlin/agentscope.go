package snippets

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/a2a"
)

// TestA2ADocSnippetsRoundTrip proves the documented A2A server + client
// snippets actually work together — the docs example is executed, not just
// compiled (23.5).
func TestA2ADocSnippetsRoundTrip(t *testing.T) {
	card := a2a.AgentCard{
		Name:    "coder",
		URL:     "http://test",
		Version: "1.0.0",
	}
	server := a2a.NewServer(card, a2a.NewAgentAdapter(stubAgent{agentName: "coder"}), nil)
	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx := context.Background()
	client := a2a.NewHTTPClient(ts.URL)
	reply, err := client.Send(ctx, &a2a.Message{
		Role:    "user",
		Content: "Write a Go function that reverses a string.",
	})
	if err != nil {
		t.Fatalf("documented Send snippet failed: %v", err)
	}
	if reply == nil {
		t.Fatal("expected a reply message")
	}
	if got := replyText(reply); got == "" {
		t.Fatal("expected non-empty reply content")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func replyText(m *a2a.Message) string {
	if m == nil {
		return ""
	}
	return m.Content
}
