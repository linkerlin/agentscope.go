// model/moonshot/kimi_test.go — 20.4: the unified thinking option is
// serialized correctly by the Kimi backend on BOTH the plain and streaming
// paths; Kimi cards load; reasoning_content parsing is reused from the Ark
// wire types (same OpenAI-compatible shape).
package moonshot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func newKimi(t *testing.T, handler http.HandlerFunc) *KimiChatModel {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m, err := KimiBuilder("kimi-key").BaseURL(srv.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

var oneUserMsg = []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("你好").Build()}

// TestKimiChat_ThinkingUnifiedOption: WithThinking serializes as the Kimi
// `thinking` parameter on the PLAIN path; unset = absent from the wire.
func TestKimiChat_ThinkingUnifiedOption(t *testing.T) {
	var bodies []map[string]any
	m := newKimi(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	})
	ctx := context.Background()

	if _, err := m.Chat(ctx, oneUserMsg); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Chat(ctx, oneUserMsg, model.WithThinking(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Chat(ctx, oneUserMsg, model.WithThinking(false)); err != nil {
		t.Fatal(err)
	}

	if _, has := bodies[0]["thinking"]; has {
		t.Fatalf("unset thinking must stay off the wire: %v", bodies[0]["thinking"])
	}
	on, _ := bodies[1]["thinking"].(map[string]any)
	if on["type"] != "enabled" {
		t.Fatalf("WithThinking(true): %v", bodies[1]["thinking"])
	}
	off, _ := bodies[2]["thinking"].(map[string]any)
	if off["type"] != "disabled" {
		t.Fatalf("WithThinking(false): %v", bodies[2]["thinking"])
	}
	if bodies[1]["model"] != ModelKimiK3 {
		t.Fatalf("model: %v", bodies[1]["model"])
	}
}

// TestKimiStream_ThinkingUnifiedOption: the SAME serialization on the
// streaming path, plus reasoning deltas stream as IsThinking chunks.
func TestKimiStream_ThinkingUnifiedOption(t *testing.T) {
	var streamBody map[string]any
	m := newKimi(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &streamBody)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"reasoning_content":"想"}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"content":"答"}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	ch, err := m.ChatStream(context.Background(), oneUserMsg, model.WithThinking(true))
	if err != nil {
		t.Fatal(err)
	}
	th, _ := streamBody["thinking"].(map[string]any)
	if th["type"] != "enabled" {
		t.Fatalf("stream thinking wire: %v", streamBody["thinking"])
	}
	if streamBody["stream"] != true {
		t.Fatalf("stream flag: %v", streamBody["stream"])
	}
	var thinking, text string
	var usage *model.ChatUsage
	for c := range ch {
		if c.IsThinking {
			thinking += c.Delta
		} else if c.Delta != "" {
			text += c.Delta
		}
		if c.Done && c.Usage != nil {
			usage = c.Usage
		}
	}
	if thinking != "想" || text != "答" {
		t.Fatalf("stream contents: thinking=%q text=%q", thinking, text)
	}
	if usage == nil || usage.TotalTokens != 2 {
		t.Fatalf("usage: %+v", usage)
	}
}

// TestKimiChat_ReasoningBlocksSurvive: Kimi responses carry reasoning_content
// alongside content — the Ark wire-type parsing keeps both (no dropped
// blocks on the Kimi path either).
func TestKimiChat_ReasoningBlocksSurvive(t *testing.T) {
	m := newKimi(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"答案","reasoning_content":"推理"}}]}`))
	})
	msg, err := m.Chat(context.Background(), oneUserMsg, model.WithThinking(true))
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetThinkingContent() != "推理" || msg.GetTextContent() != "答案" {
		t.Fatalf("blocks dropped: thinking=%q text=%q", msg.GetThinkingContent(), msg.GetTextContent())
	}
}

// TestKimiChat_Error: the error envelope surfaces on the wire error.
func TestKimiChat_Error(t *testing.T) {
	m := newKimi(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limit","message":"too many requests"}}`))
	})
	_, err := m.Chat(context.Background(), oneUserMsg)
	if err == nil || !strings.Contains(err.Error(), "rate_limit") {
		t.Fatalf("error: %v", err)
	}
}

// TestKimiBuilder_DefaultsAndCards: builder defaults target kimi-k3; the
// K3 and K2.7 cards both load.
func TestKimiBuilder_DefaultsAndCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	m, err := KimiBuilder("k").BaseURL(srv.URL).ModelName(ModelKimiK27).Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != ModelKimiK27 {
		t.Fatalf("model: %s", m.ModelName())
	}
	if _, err := m.Chat(context.Background(), oneUserMsg); err != nil {
		t.Fatal(err)
	}

	cards, err := model.LoadModelCardsFromDir("../cards")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, c := range cards {
		if c.Provider == "moonshot" {
			have[c.ID] = true
		}
	}
	if !have["kimi-k3"] || !have["kimi-k2.7"] {
		t.Fatalf("kimi cards missing: %v", have)
	}
}
