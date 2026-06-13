package memory

import (
	"context"
	"fmt"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func BenchmarkEmbeddingCacheHit(b *testing.B) {
	base := &countingEmbedder{v: []float32{0.1, 0.2, 0.3}}
	cache := NewEmbeddingCache(base, 1024)
	ctx := context.Background()
	// prime cache
	_, _ = cache.Embed(ctx, "hello")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Embed(ctx, "hello")
	}
}

func BenchmarkEmbeddingCacheMiss(b *testing.B) {
	base := &countingEmbedder{v: []float32{0.1, 0.2, 0.3}}
	cache := NewEmbeddingCache(base, 1024)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Embed(ctx, fmt.Sprintf("hello %d", i))
	}
}

func BenchmarkFTSIndexSearch(b *testing.B) {
	dir := b.TempDir()
	idx, err := NewFTSIndex(dir + "/fts.db")
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()
	for i := 0; i < 100; i++ {
		_ = idx.Insert(NewMemoryNode(MemoryTypePersonal, "u", fmt.Sprintf("doc number %d content", i)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Search("content", 10, nil, "")
	}
}

func BenchmarkRankMemoryNodesHybrid(b *testing.B) {
	e := fixedEmbed{dim: 4}
	dir := b.TempDir()
	idx, err := NewFTSIndex(dir + "/fts.db")
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()
	var nodes []*MemoryNode
	for i := 0; i < 20; i++ {
		n := NewMemoryNode(MemoryTypePersonal, "u", fmt.Sprintf("document %d", i))
		v, _ := e.Embed(context.Background(), n.Content)
		n.Vector = v
		nodes = append(nodes, n)
		_ = idx.Insert(n)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RankMemoryNodesHybrid(nodes, "document", 0.5, idx)
	}
}

func BenchmarkReMeFileMemoryAdd(b *testing.B) {
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = b.TempDir()
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("benchmark message").Build()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Add(msg)
	}
}
