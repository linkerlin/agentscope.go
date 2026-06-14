// examples/rag/main.go
//
// Demo: Minimal RAG pipeline — load text, embed, store in vector memory, retrieve.
//
// This demo shows loader.NewTextLoader, embedding.NewOpenAI (stub),
// memory.NewReMeVectorMemory, and semantic retrieval. No real API key is needed.
//
// How to run:
//   cd examples/rag && go run main.go

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/loader"
	"github.com/linkerlin/agentscope.go/memory"
)

// stubEmbedder is a mock embedder for demo purposes.
type stubEmbedder struct{}

func (s *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{float32(len(text)), 0.2, 0.3, 0.4}, nil
}
func (s *stubEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for _, t := range texts {
		v, _ := s.Embed(ctx, t)
		out = append(out, v)
	}
	return out, nil
}

func main() {
	ctx := context.Background()

	// 1. Create a temporary text file to load.
	tmpFile := "demo_doc.txt"
	_ = os.WriteFile(tmpFile, []byte("Agentscope is a multi-agent platform.\nIt supports Go, Python, and more."), 0o644)
	defer os.Remove(tmpFile)

	// 2. Load the document with TextLoader.
	textLoader := &loader.TextLoader{}
	docs, err := textLoader.Load(ctx, tmpFile)
	if err != nil {
		fmt.Println("load error:", err)
		return
	}
	fmt.Printf("loaded %d document(s)\n", len(docs))

	// 3. Create a mock embedder (use embedding.NewOpenAI("sk-xxx", "") for real calls).
	embedder := &stubEmbedder{}

	// 4. Create vector memory with local store.
	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = ".cache/rag_demo"
	vm, err := memory.NewReMeVectorMemory(cfg, memory.NewSimpleTokenCounter(), nil, embedder)
	if err != nil {
		fmt.Println("vector memory error:", err)
		return
	}
	defer vm.Close()

	// 5. Add documents to vector memory.
	for i, doc := range docs {
		node := memory.NewMemoryNode(memory.MemoryTypeHistory, "rag-demo", doc.Content)
		node.Metadata = doc.Metadata
		if err := vm.AddMemory(ctx, node); err != nil {
			fmt.Printf("add memory %d error: %v\n", i, err)
		}
	}
	fmt.Println("added documents to vector memory")

	// 6. Retrieve relevant chunks for a query.
	query := "What is Agentscope?"
	results, err := vm.RetrieveMemory(ctx, query, memory.RetrieveOptions{TopK: 3, MinScore: 0.0})
	if err != nil {
		fmt.Println("retrieve error:", err)
		return
	}
	fmt.Printf("query=%q results=%d\n", query, len(results))
	for _, r := range results {
		fmt.Printf("  score=%.3f content=%q\n", r.Score, r.Content)
	}
}
