package memory

import (
	"context"
)

// EmbeddingModel 文本嵌入（向量记忆依赖）
type EmbeddingModel interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Embedder 仅实现单条嵌入时的批量默认实现
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// BatchFromEmbedder 将仅实现 Embed 的类型包装为 EmbeddingModel
func BatchFromEmbedder(e Embedder) EmbeddingModel {
	return batchEmbedder{e: e}
}

type batchEmbedder struct{ e Embedder }

func (b batchEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return b.e.Embed(ctx, text)
}

func (b batchEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := b.e.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}
