package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
)

type stubEmbeddingModel struct {
	name string
	dim  int
}

func (s *stubEmbeddingModel) ModelName() string { return s.name }
func (s *stubEmbeddingModel) Dimensions() int   { return s.dim }
func (s *stubEmbeddingModel) Embed(_ context.Context, input []string) (*model.EmbeddingResponse, error) {
	resp := &model.EmbeddingResponse{Object: "list", Model: s.name}
	for i, text := range input {
		vec := make([]float32, s.dim)
		vec[0] = float32(len(text))
		resp.Data = append(resp.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: vec,
		})
	}
	return resp, nil
}

func TestHandleCreateEmbeddings_StringInput(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"}).
		WithEmbeddingModel(&stubEmbeddingModel{name: "embed-v1", dim: 4})
	srv.RegisterEmbeddingRoutes()

	body, _ := json.Marshal(map[string]any{"input": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embeddings", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp model.EmbeddingResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 || len(resp.Data[0].Embedding) != 4 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestHandleCreateEmbeddings_MissingInput(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"}).
		WithEmbeddingModel(&stubEmbeddingModel{name: "embed-v1", dim: 4})
	srv.RegisterEmbeddingRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/embeddings", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleCreateEmbeddings_NotConfigured(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	body, _ := json.Marshal(map[string]any{"input": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embeddings", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (route not registered), got %d", rr.Code)
	}
}
