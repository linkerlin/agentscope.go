// rag_qa demonstrates a simple RAG pipeline: load documents, embed chunks, retrieve, answer.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/loader"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/openai"
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

	// 1. Load a document
	docLoader := &loader.TextLoader{}
	docs, err := docLoader.Load(context.Background(), "./knowledge.txt")
	if err != nil {
		log.Printf("failed to load knowledge.txt: %v", err)
		log.Println("Using built-in demo knowledge instead.")
		docs = []loader.Document{{
			Content: "Go is a statically typed, compiled language designed at Google. " +
				"It features goroutines and channels for concurrency, fast compile times, and a strong standard library.",
		}}
	}

	// 2. Chunk and embed into vector memory
	embedModel := embedding.NewOpenAI(openaiKey, "text-embedding-3-small")
	embedModel = embedding.WithFileCache(embedModel, "./.rag_cache")

	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = ".reme_rag"

	vectorMemory, err := memory.NewReMeVectorMemory(cfg, nil, nil, &modelEmbeddingAdapter{inner: embedModel})
	if err != nil {
		log.Fatal(err)
	}

	for _, doc := range docs {
		for _, chunk := range chunkText(doc.Content, 200) {
			node := memory.NewMemoryNode(memory.MemoryTypePersonal, "rag", chunk)
			if err := vectorMemory.AddMemory(context.Background(), node); err != nil {
				log.Fatal(err)
			}
		}
	}

	// 3. Retrieve relevant chunks
	query := "What makes Go fast to compile?"
	nodes, err := vectorMemory.RetrieveMemory(context.Background(), query, memory.RetrieveOptions{TopK: 3})
	if err != nil {
		log.Fatal(err)
	}

	var contextParts []string
	for _, n := range nodes {
		contextParts = append(contextParts, n.Content)
	}
	contextText := strings.Join(contextParts, "\n---\n")

	// 4. Answer with an Agent
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

	prompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", contextText, query)
	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent(prompt).
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Answer ===")
	fmt.Println(resp.GetTextContent())
}

func chunkText(text string, size int) []string {
	words := strings.Fields(text)
	var chunks []string
	for i := 0; i < len(words); i += size {
		end := i + size
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	return chunks
}
