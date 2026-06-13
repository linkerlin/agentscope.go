// Package rerank provides document reranking implementations for RAG pipelines.
package rerank

import "context"

// Document is a candidate document to be reranked.
type Document struct {
	ID      string
	Content string
	Score   float64 // optional initial score from vector search
}

// Result is a reranked document with its relevance score.
type Result struct {
	Document
	RelevanceScore float64
}

// Reranker reorders a list of candidate documents by relevance to a query.
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []Document, topK int) ([]Result, error)
}

// NoopReranker returns the input documents unchanged.
type NoopReranker struct{}

func (NoopReranker) Rerank(ctx context.Context, query string, docs []Document, topK int) ([]Result, error) {
	results := make([]Result, 0, len(docs))
	for _, d := range docs {
		results = append(results, Result{Document: d, RelevanceScore: d.Score})
	}
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}
