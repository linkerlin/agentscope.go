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

// DashScopeContent represents a single content item for DashScope multimodal embedding.
type DashScopeContent struct {
	Type     string `json:"type"` // "text" or "image_url"
	Text     string `json:"text,omitempty"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

// DashScopeMultimodalEmbedder generates multimodal embeddings via DashScope native API.
// Supports text + image_url inputs for models like "multimodal-embedding-v1".
type DashScopeMultimodalEmbedder struct {
	apiKey    string
	modelName string
	client    *http.Client
}

// NewDashScopeMultimodal creates a DashScope multimodal embedder.
// modelName defaults to "multimodal-embedding-v1".
func NewDashScopeMultimodal(apiKey, modelName string) *DashScopeMultimodalEmbedder {
	if modelName == "" {
		modelName = "multimodal-embedding-v1"
	}
	return &DashScopeMultimodalEmbedder{
		apiKey:    apiKey,
		modelName: modelName,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (e *DashScopeMultimodalEmbedder) ModelName() string { return e.modelName }
func (e *DashScopeMultimodalEmbedder) Dimensions() int   { return 1536 }

// Embed implements model.EmbeddingModel for text-only batches.
// Each string is wrapped as a text content item.
func (e *DashScopeMultimodalEmbedder) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	if len(input) == 0 {
		return &model.EmbeddingResponse{Object: "list", Model: e.modelName}, nil
	}
	contents := make([][]DashScopeContent, len(input))
	for i, text := range input {
		contents[i] = []DashScopeContent{{Type: "text", Text: text}}
	}
	return e.embedContents(ctx, contents)
}

// EmbedContents generates embeddings for arbitrary multimodal content batches.
// Each batch item can contain multiple text/image contents.
func (e *DashScopeMultimodalEmbedder) EmbedContents(ctx context.Context, contents [][]DashScopeContent) (*model.EmbeddingResponse, error) {
	if len(contents) == 0 {
		return &model.EmbeddingResponse{Object: "list", Model: e.modelName}, nil
	}
	return e.embedContents(ctx, contents)
}

func (e *DashScopeMultimodalEmbedder) embedContents(ctx context.Context, contents [][]DashScopeContent) (*model.EmbeddingResponse, error) {
	reqBody := map[string]any{
		"model": e.modelName,
		"input": map[string]any{
			"contents": contents,
		},
		"parameters": map[string]any{
			"auto_truncation": true,
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope multimodal embed: %s: %s", resp.Status, string(raw))
	}

	var parsed struct {
		Output struct {
			Embeddings []struct {
				TextIndex  int       `json:"text_index"`
				Embedding  []float32 `json:"embedding"`
				Similarity float64   `json:"similarity"`
			} `json:"embeddings"`
		} `json:"output"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	respModel := &model.EmbeddingResponse{
		Object: "list",
		Model:  e.modelName,
		Usage:  model.EmbeddingUsage{TotalTokens: parsed.Usage.TotalTokens},
	}
	for i, emb := range parsed.Output.Embeddings {
		respModel.Data = append(respModel.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: emb.Embedding,
		})
	}
	return respModel, nil
}

// AsModel returns the embedder as model.EmbeddingModel.
func (e *DashScopeMultimodalEmbedder) AsModel() model.EmbeddingModel { return e }

var _ model.EmbeddingModel = (*DashScopeMultimodalEmbedder)(nil)
