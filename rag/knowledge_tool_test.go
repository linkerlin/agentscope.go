package rag

import (
	"context"
	"strings"
	"testing"
)

func TestKnowledgeRetrievalTool_Execute(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	rag := NewSimpleMemoryRAG(emb)
	ctx := context.Background()
	_ = rag.Store(ctx, "doc1", "Go is a programming language")

	tool := NewKnowledgeRetrievalTool(rag, 1)
	resp, err := tool.Execute(ctx, map[string]any{"query": "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Go is a programming language") {
		t.Fatalf("unexpected result: %s", resp.GetTextContent())
	}
}

func TestKnowledgeRetrievalTool_ExecuteWithTopK(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	rag := NewSimpleMemoryRAG(emb)
	ctx := context.Background()
	_ = rag.Store(ctx, "doc1", "text one")
	_ = rag.Store(ctx, "doc2", "text two")

	tool := NewKnowledgeRetrievalTool(rag, 2)
	resp, err := tool.Execute(ctx, map[string]any{"query": "text", "top_k": 2})
	if err != nil {
		t.Fatal(err)
	}
	content := resp.GetTextContent()
	if !strings.Contains(content, "[1]") {
		t.Fatalf("expected numbered results, got: %s", content)
	}
}

func TestKnowledgeRetrievalTool_MissingQuery(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	tool := NewKnowledgeRetrievalTool(NewSimpleMemoryRAG(emb), 3)
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query required error, got %v", err)
	}
}

func TestKnowledgeRetrievalTool_NoResults(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	rag := NewSimpleMemoryRAG(emb)
	tool := NewKnowledgeRetrievalTool(rag, 3)
	resp, err := tool.Execute(context.Background(), map[string]any{"query": "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "No relevant knowledge found." {
		t.Fatalf("unexpected result: %s", resp.GetTextContent())
	}
}
