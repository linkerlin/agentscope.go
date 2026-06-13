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

// CohereReranker uses Cohere's /rerank API to reorder documents.
type CohereReranker struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewCohereReranker creates a Cohere reranker.
// model defaults to "rerank-english-v3.0".
func NewCohereReranker(apiKey, model string) *CohereReranker {
	if model == "" {
		model = "rerank-english-v3.0"
	}
	return &CohereReranker{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.cohere.com/v2/rerank",
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (r *CohereReranker) Rerank(ctx context.Context, query string, docs []Document, topK int) ([]Result, error) {
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
		"model":            r.model,
		"query":            query,
		"documents":        texts,
		"top_n":            topK,
		"return_documents": false,
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
		return nil, fmt.Errorf("cohere rerank: %s: %s", resp.Status, string(raw))
	}
	var parsed struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
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

var _ Reranker = (*CohereReranker)(nil)
