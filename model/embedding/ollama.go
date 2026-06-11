package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/model"
)

// OllamaEmbedder calls Ollama's /api/embed endpoint (OpenAI-compatible /v1/embeddings fallback).
type OllamaEmbedder struct {
	baseURL    string
	modelName  string
	dimension  int
	httpClient *http.Client
}

// NewOllamaEmbedder creates an embedder for a local Ollama server.
func NewOllamaEmbedder(baseURL, modelName string, dimension int) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &OllamaEmbedder{
		baseURL:    strings.TrimRight(baseURL, "/"),
		modelName:  modelName,
		dimension:  dimension,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (e *OllamaEmbedder) ModelName() string { return e.modelName }
func (e *OllamaEmbedder) Dimensions() int   { return e.dimension }

// Embed generates embeddings via Ollama.
func (e *OllamaEmbedder) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	if len(input) == 0 {
		return &model.EmbeddingResponse{Object: "list", Model: e.modelName}, nil
	}
	vecs, err := e.embedOllama(ctx, input)
	if err != nil {
		return nil, err
	}
	resp := &model.EmbeddingResponse{Object: "list", Model: e.modelName}
	for i, vec := range vecs {
		resp.Data = append(resp.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: vec,
		})
	}
	return resp, nil
}

func (e *OllamaEmbedder) embedOllama(ctx context.Context, input []string) ([][]float32, error) {
	body := map[string]any{
		"model": e.modelName,
		"input": input,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed: %s: %s", resp.Status, string(raw))
	}
	var parsed struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([][]float32, len(parsed.Embeddings))
	for i, vec := range parsed.Embeddings {
		f32 := make([]float32, len(vec))
		for j, v := range vec {
			f32[j] = float32(v)
		}
		out[i] = f32
	}
	return out, nil
}

// Embed implements memory.EmbeddingModel for reuse in memory pipelines.
func (e *OllamaEmbedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("ollama embed: empty response")
	}
	return resp.Data[0].Embedding, nil
}

// EmbedBatch implements memory.EmbeddingModel.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return e.embedOllama(ctx, texts)
}

// AsModel returns the embedder as model.EmbeddingModel.
func (e *OllamaEmbedder) AsModel() model.EmbeddingModel { return e }

var _ model.EmbeddingModel = (*OllamaEmbedder)(nil)
