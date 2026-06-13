// rag_with_rerank demonstrates a two-stage RAG pipeline:
// 1. Vector retrieval recalls a candidate set.
// 2. Reranker (Cohere / Jina / Local) reorders candidates by relevance.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/rerank"
)

// modelEmbeddingAdapter adapts model.EmbeddingModel to memory.EmbeddingModel.
type modelEmbeddingAdapter struct {
	inner model.EmbeddingModel
}

func (a *modelEmbeddingAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := a.inner.Embed(ctx, []string{text})
	if err != nil || len(resp.Data) == 0 {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}

func (a *modelEmbeddingAdapter) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := a.inner.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

func main() {
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	embedModel := embedding.NewOpenAI(openaiKey, "text-embedding-3-small")
	embedModel = embedding.WithFileCache(embedModel, "./.rag_cache")

	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = ".reme_rag_rerank"

	vectorMemory, err := memory.NewReMeVectorMemory(cfg, nil, nil, &modelEmbeddingAdapter{inner: embedModel})
	if err != nil {
		log.Fatal(err)
	}

	// Attach reranker: prefer Cohere or Jina if API key is set, otherwise use local cosine rerank.
	var rk rerank.Reranker
	if key := os.Getenv("COHERE_API_KEY"); key != "" {
		rk = rerank.NewCohereReranker(key, "")
		fmt.Println("Using Cohere reranker")
	} else if key := os.Getenv("JINA_API_KEY"); key != "" {
		rk = rerank.NewJinaReranker(key, "")
		fmt.Println("Using Jina reranker")
	} else {
		rk = rerank.NewLocalReranker(&modelEmbeddingAdapter{inner: embedModel})
		fmt.Println("Using local cosine reranker")
	}
	vectorMemory = vectorMemory.WithReranker(rk)

	ctx := context.Background()
	docs := []string{
		"Go is a statically typed, compiled language designed at Google.",
		"Python is a dynamically typed language widely used in data science.",
		"Go features goroutines and channels for lightweight concurrency.",
		"Kubernetes is a container orchestration platform written in Go.",
		"JavaScript runs in browsers and on servers via Node.js.",
	}
	for _, doc := range docs {
		node := memory.NewMemoryNode(memory.MemoryTypePersonal, "rag", doc)
		if err := vectorMemory.AddMemory(ctx, node); err != nil {
			log.Fatal(err)
		}
	}

	query := "How does Go handle concurrency?"

	// Compare without reranker.
	withoutRerank, err := vectorMemory.RetrieveMemory(ctx, query, memory.RetrieveOptions{TopK: 3})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== Without reranker (vector search) ===")
	for _, n := range withoutRerank {
		fmt.Printf("- %.4f %s\n", n.Score, n.Content)
	}

	// The WithReranker memory already applies reranker; to show the difference we re-query with a
	// fresh memory that has no reranker.
	plainMemory, err := memory.NewReMeVectorMemory(cfg, nil, nil, &modelEmbeddingAdapter{inner: embedModel})
	if err != nil {
		log.Fatal(err)
	}
	for _, doc := range docs {
		_ = plainMemory.AddMemory(ctx, memory.NewMemoryNode(memory.MemoryTypePersonal, "rag", doc))
	}
	withRerank, err := vectorMemory.RetrieveMemory(ctx, query, memory.RetrieveOptions{TopK: 3})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== With reranker ===")
	for _, n := range withRerank {
		fmt.Printf("- %.4f %s\n", n.Score, n.Content)
	}

	// Answer with an Agent using reranked context.
	chatModel, err := openai.Builder().APIKey(openaiKey).ModelName("gpt-4o-mini").Build()
	if err != nil {
		log.Fatal(err)
	}
	agent, err := react.Builder().
		Name("RAGAssistant").
		SysPrompt("You answer questions based ONLY on the provided context.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	var contextParts []string
	for _, n := range withRerank {
		contextParts = append(contextParts, n.Content)
	}
	prompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", strings.Join(contextParts, "\n---\n"), query)
	resp, err := agent.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== Answer ===")
	fmt.Println(resp.GetTextContent())
}
