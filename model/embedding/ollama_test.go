package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbedder_Embed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Model != "test-model" {
			t.Errorf("unexpected model: %s", body.Model)
		}
		vecs := make([][]float64, len(body.Input))
		for i := range body.Input {
			vecs[i] = []float64{0.1, 0.2}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": vecs})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model", 2)
	resp, err := e.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(resp.Data))
	}
	if resp.Data[0].Index != 0 || len(resp.Data[0].Embedding) != 2 {
		t.Fatalf("unexpected first embedding: %#v", resp.Data[0])
	}
}

func TestOllamaEmbedder_EmptyInput(t *testing.T) {
	e := NewOllamaEmbedder("", "m", 0)
	resp, err := e.Embed(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data, got %#v", resp.Data)
	}
}
