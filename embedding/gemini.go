package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/model"
)

const geminiDefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// NewGemini creates a Gemini embedding model.
// modelName defaults to "text-embedding-004" if empty.
// Supports text embeddings via embedContent API.
func NewGemini(apiKey, modelName string) model.EmbeddingModel {
	return NewGeminiWithDims(apiKey, modelName, 0)
}

// NewGeminiWithDims creates a Gemini embedding model with explicit dimension override.
// Pass 0 to use the model's default (768 for text-embedding-004).
func NewGeminiWithDims(apiKey, modelName string, dims int) model.EmbeddingModel {
	if modelName == "" {
		modelName = "text-embedding-004"
	}
	return &geminiModel{
		apiKey:    apiKey,
		modelName: modelName,
		baseURL:   geminiDefaultBaseURL,
		dims:      dims,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

type geminiModel struct {
	apiKey    string
	modelName string
	baseURL   string
	dims      int
	client    *http.Client
}

func (m *geminiModel) ModelName() string { return m.modelName }
func (m *geminiModel) Dimensions() int {
	if m.dims > 0 {
		return m.dims
	}
	// text-embedding-004 is 768 dim typically
	return 768
}

func (m *geminiModel) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	if len(input) == 0 {
		return &model.EmbeddingResponse{Object: "list", Model: m.modelName}, nil
	}

	resp := &model.EmbeddingResponse{
		Object: "list",
		Model:  m.modelName,
	}

	for i, text := range input {
		vec, err := m.embedSingle(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("gemini embed %d: %w", i, err)
		}
		resp.Data = append(resp.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: vec,
		})
	}
	return resp, nil
}

func (m *geminiModel) embedSingle(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", m.baseURL, m.modelName, m.apiKey)

	body := map[string]any{
		"model": fmt.Sprintf("models/%s", m.modelName),
		"content": map[string]any{
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	raw, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini embed: %s: %s", httpResp.Status, string(raw))
	}

	var parsed struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("gemini parse: %w", err)
	}
	return parsed.Embedding.Values, nil
}

var _ model.EmbeddingModel = (*geminiModel)(nil)
