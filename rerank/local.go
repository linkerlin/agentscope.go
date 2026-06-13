package rerank

import (
	"context"
	"sort"

	"github.com/linkerlin/agentscope.go/memory/vector"
)

// LocalReranker is a lightweight reranker that uses cosine similarity between
// query embedding and document embeddings. It does not require an external API.
type LocalReranker struct {
	embed vector.EmbeddingModel
}

// NewLocalReranker creates a local reranker using the given embedding model.
func NewLocalReranker(embed vector.EmbeddingModel) *LocalReranker {
	return &LocalReranker{embed: embed}
}

func (r *LocalReranker) Rerank(ctx context.Context, query string, docs []Document, topK int) ([]Result, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = len(docs)
	}
	qvec, err := r.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(docs))
	for _, d := range docs {
		// Use pre-existing vector if the caller attached it via Metadata.
		var dvec []float32
		if dvec == nil {
			// Fallback: embed the document content on the fly.
			v, err := r.embed.Embed(ctx, d.Content)
			if err != nil {
				continue
			}
			dvec = v
		}
		score := vector.CosineSimilarity(qvec, dvec)
		results = append(results, Result{
			Document:       d,
			RelevanceScore: score,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

var _ Reranker = (*LocalReranker)(nil)
