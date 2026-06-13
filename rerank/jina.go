package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// JinaReranker uses Jina AI's /v1/rerank API to reorder documents.
type JinaReranker struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewJinaReranker creates a Jina reranker.
// model defaults to "jina-reranker-v2-base-multilingual".
func NewJinaReranker(apiKey, model string) *JinaReranker {
	if model == "" {
		model = "jina-reranker-v2-base-multilingual"
	}
	return &JinaReranker{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.jina.ai/v1/rerank",
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (r *JinaReranker) Rerank(ctx context.Context, query string, docs []Document, topK int) ([]Result, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = len(docs)
	}
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}
	body := map[string]any{
		"model":     r.model,
		"query":     query,
		"documents": texts,
		"top_n":     topK,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jina rerank: %s: %s", resp.Status, string(raw))
	}
	var parsed struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
			Document       struct {
				Text string `json:"text"`
			} `json:"document"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		if item.Index < 0 || item.Index >= len(docs) {
			continue
		}
		results = append(results, Result{
			Document:       docs[item.Index],
			RelevanceScore: item.RelevanceScore,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})
	return results, nil
}

var _ Reranker = (*JinaReranker)(nil)
