package tts_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tts"
)

func TestDashScope_Synthesize(t *testing.T) {
	audioBytes := []byte{0x52, 0x49, 0x46, 0x46} // "RIFF"-ish wav header bytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/services/aigc/text2audio/generation" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		body, _ := io.ReadAll(r.Body)
		var req ttsDashReq
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "cosyvoice-v1" || req.Input.Text != "hello" {
			t.Fatalf("unexpected request: %+v", req)
		}
		if req.Parameters.Voice != "longxiaochun" || req.Parameters.Format != "wav" {
			t.Fatalf("unexpected params: %+v", req.Parameters)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"audio":      base64.StdEncoding.EncodeToString(audioBytes),
				"request_id": "req-1",
			},
			"request_id": "req-1",
		})
	}))
	defer srv.Close()

	m := tts.NewDashScope("test-key").
		WithBaseURL(srv.URL).
		WithHTTPClient(srv.Client())
	resp, err := m.Synthesize(context.Background(), "hello", tts.Options{Format: "wav"})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if string(resp.Audio) != string(audioBytes) {
		t.Fatalf("audio mismatch: %v", resp.Audio)
	}
	if resp.MediaType != "audio/wav" || !resp.IsLast {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDashScope_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"InvalidApiKey","message":"bad key"}`))
	}))
	defer srv.Close()

	m := tts.NewDashScope("bad").WithBaseURL(srv.URL).WithHTTPClient(srv.Client())
	if _, err := m.Synthesize(context.Background(), "hi", tts.Options{}); err == nil {
		t.Fatal("expected error on API error")
	}
}

func TestDashScope_MissingKey(t *testing.T) {
	m := tts.NewDashScope("")
	if _, err := m.Synthesize(context.Background(), "hi", tts.Options{}); err == nil {
		t.Fatal("expected error when api key missing")
	}
}

// ttsDashReq mirrors the DashScope request body for assertions.
type ttsDashReq struct {
	Model string `json:"model"`
	Input struct {
		Text string `json:"text"`
	} `json:"input"`
	Parameters struct {
		Voice  string `json:"voice"`
		Format string `json:"format"`
	} `json:"parameters"`
}

func TestOpenAIAdapter_Synthesize(t *testing.T) {
	audioBytes := []byte{0x01, 0x02, 0x03, 0x04}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/speech" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer oai-key" {
			t.Fatalf("unexpected auth: %s", got)
		}
		_, _ = w.Write(audioBytes)
	}))
	defer srv.Close()

	backend := model.NewOpenAITTS("oai-key").
		WithBaseURL(srv.URL).
		WithModel("tts-1").
		WithVoice("alloy").
		WithHTTPClient(srv.Client())
	adapter := tts.NewOpenAIAdapter(backend).WithDefaults(tts.Options{Format: "mp3"})

	resp, err := adapter.Synthesize(context.Background(), "hello world", tts.Options{})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if string(resp.Audio) != string(audioBytes) {
		t.Fatalf("audio mismatch: %v", resp.Audio)
	}
	if resp.MediaType != "audio/mpeg" || !resp.IsLast {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if adapter.ModelName() != "tts-1" {
		t.Fatalf("unexpected model name: %s", adapter.ModelName())
	}
}

func TestListModelCards(t *testing.T) {
	cards, err := tts.ListModelCards()
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) < 4 {
		t.Fatalf("expected at least 4 cards, got %d", len(cards))
	}
	ids := map[string]bool{}
	for _, c := range cards {
		ids[c.ID] = true
		if c.Provider == "" || c.Model == "" {
			t.Fatalf("card missing provider/model: %+v", c)
		}
	}
	for _, want := range []string{"cosyvoice-v1", "qwen3-tts-flash", "qwen3-tts-flash-realtime", "openai-tts-1", "openai-tts-1-hd"} {
		if !ids[want] {
			t.Fatalf("missing card: %s", want)
		}
	}
}

func TestFindModelCard(t *testing.T) {
	c := tts.FindModelCard("openai-tts-1-hd")
	if c == nil || c.Model != "tts-1-hd" {
		t.Fatalf("find card failed: %+v", c)
	}
	if tts.FindModelCard("does-not-exist") != nil {
		t.Fatal("expected nil for unknown card")
	}
}
