// Command rag_kb demonstrates the full RAG indexing pipeline end-to-end:
// register parsers → create a knowledge base → index documents
// (blob → parse → chunk → embed → insert) → search → print results.
//
// Runs offline using a deterministic stub embedder (no API key needed).
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/index"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

// deterministic embedder: maps text to a stable 8-dim vector so the demo
// runs without network access. Replace with embedding.NewOpenAI(...) in
// production.
type demoEmbedder struct{}

func (demoEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, 8)
	for i := range v {
		v[i] = float32(strings.Count(text, string(rune('a'+i))) + 1)
	}
	return v, nil
}

var docs = []struct {
	id, source, body string
}{
	{"d1", "onboarding.md", "# New Hire Onboarding\nWelcome! Your PTO is 15 days/year. Submit requests via the HR portal."},
	{"d2", "security.md", "# Security Policy\nReport vulnerabilities to security@corp.com. Rotate API keys every 90 days."},
	{"d3", "expenses.md", "# Expense Reimbursement\nSubmit receipts within 30 days. Manager approval required for amounts over $500."},
}

func main() {
	ctx := context.Background()

	// 1. Set up the pipeline components.
	bs, err := blob.NewLocalBlobStore("./.rag_demo_blobs")
	if err != nil {
		log.Fatal(err)
	}
	reg := parser.NewRegistry(parser.NewTextParser())
	ch := chunker.NewApproxTokenChunker()
	mgr := kb.NewCollectionPerKBManager(kb.NewInMemoryVectorStore(), func(string) (kb.Embedder, error) {
		return demoEmbedder{}, nil
	})

	// 2. Create a knowledge base.
	if err := mgr.Create(ctx, kb.Spec{Name: "handbook", Description: "Company handbook"}); err != nil {
		log.Fatal(err)
	}

	// 3. Index each document through the pipeline.
	worker := &index.Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr,
		OnStatus: func(s index.Status) {
			if s.Err != nil {
				fmt.Printf("  [FAIL] %s: %v\n", s.Task.DocID, s.Err)
			} else {
				fmt.Printf("  [OK]   %s: %d chunks indexed\n", s.Task.DocID, s.Chunks)
			}
		},
	}
	fmt.Println("Indexing documents...")
	for _, d := range docs {
		uri, err := bs.Put(ctx, d.source, strings.NewReader(d.body))
		if err != nil {
			log.Fatal(err)
		}
		if err := worker.Process(ctx, index.Task{
			KBName: "handbook", DocID: d.id, BlobURI: uri,
			MediaType: "text/markdown", Source: d.source,
		}); err != nil {
			log.Fatal(err)
		}
	}

	// 4. Search the knowledge base.
	hb, err := mgr.Get(ctx, "handbook")
	if err != nil {
		log.Fatal(err)
	}
	for _, q := range []string{"How many PTO days?", "report a security issue", "expense limit"} {
		fmt.Printf("\nQuery: %q\n", q)
		results, err := hb.Search(ctx, q, 2)
		if err != nil {
			log.Fatal(err)
		}
		for i, r := range results {
			fmt.Printf("  [%d] (score %.2f) %s …\n", i+1, r.Score, truncate(r.Text, 60))
		}
	}

	fmt.Println("\nKnowledge base documents:")
	dl, _ := hb.ListDocuments(ctx)
	for _, di := range dl {
		fmt.Printf("  - %s (%s): %d chunks\n", di.DocID, di.Source, di.Chunks)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
