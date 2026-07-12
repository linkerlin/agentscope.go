package index

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

type stubEmbedder struct{ dim int }

func (e *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.dim <= 0 {
		e.dim = 3
	}
	v := make([]float32, e.dim)
	for i := 0; i < e.dim; i++ {
		v[i] = float32(len(text)*(i+1)) / 10
	}
	return v, nil
}

func newPipeline(t *testing.T) (*blob.LocalBlobStore, *parser.Registry, chunker.Chunker, *kb.KBManager) {
	t.Helper()
	bs, err := blob.NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry(parser.NewTextParser())
	ch := chunker.NewApproxTokenChunker()
	mgr := kb.NewCollectionPerKBManager(kb.NewInMemoryVectorStore(), func(string) (kb.Embedder, error) {
		return &stubEmbedder{dim: 3}, nil
	})
	if err := mgr.Create(context.Background(), kb.Spec{Name: "docs", EmbedderID: "stub"}); err != nil {
		t.Fatal(err)
	}
	return bs, reg, ch, mgr
}

func TestWorker_ProcessTextDoc(t *testing.T) {
	bs, reg, ch, mgr := newPipeline(t)
	ctx := context.Background()

	uri, err := bs.Put(ctx, "doc1.txt", strings.NewReader("hello knowledge base"))
	if err != nil {
		t.Fatal(err)
	}

	var lastStatus Status
	w := &Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr,
		OnStatus: func(s Status) { lastStatus = s }}

	if err := w.Process(ctx, Task{
		KBName: "docs", DocID: "doc1", BlobURI: uri,
		MediaType: "text/plain", Source: "doc1.txt",
	}); err != nil {
		t.Fatal(err)
	}
	if lastStatus.Chunks == 0 {
		t.Fatal("expected at least 1 chunk indexed")
	}

	kbh, _ := mgr.Get(ctx, "docs")
	results, err := kbh.Search(ctx, "hello", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results after indexing")
	}
	if !strings.Contains(results[0].Text, "hello") {
		t.Fatalf("unexpected top result: %q", results[0].Text)
	}
}

func TestWorker_ProcessMissingKB(t *testing.T) {
	bs, reg, ch, mgr := newPipeline(t)
	w := &Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr}
	if err := w.Process(context.Background(), Task{KBName: "ghost"}); err == nil {
		t.Fatal("expected error for missing KB")
	}
}

func TestWorker_ProcessBadBlob(t *testing.T) {
	bs, reg, ch, mgr := newPipeline(t)
	w := &Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr}
	err := w.Process(context.Background(), Task{KBName: "docs", BlobURI: "local://nope.txt", MediaType: "text/plain"})
	if err == nil {
		t.Fatal("expected error for missing blob")
	}
}

func TestWorker_ProcessUnsupportedMedia(t *testing.T) {
	bs, reg, ch, mgr := newPipeline(t)
	ctx := context.Background()
	uri, _ := bs.Put(ctx, "x.bin", strings.NewReader("data"))
	w := &Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr}
	if err := w.Process(ctx, Task{KBName: "docs", BlobURI: uri, MediaType: "application/octet-stream"}); err == nil {
		t.Fatal("expected error for unsupported media type")
	}
}

func TestQueue_RunAndDrain(t *testing.T) {
	bs, reg, ch, mgr := newPipeline(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := &Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr}
	q := NewQueue(w, 8)

	processed := 0
	w.OnStatus = func(s Status) {
		if s.Err == nil {
			processed++
		}
	}

	for i := 0; i < 3; i++ {
		uri, _ := bs.Put(ctx, "d.txt", strings.NewReader("doc body "+string(rune('a'+i))))
		if err := q.SubmitCtx(ctx, Task{KBName: "docs", DocID: "d", BlobURI: uri, MediaType: "text/plain", Source: "d.txt"}); err != nil {
			t.Fatal(err)
		}
	}

	// Run in background, cancel after a short delay.
	done := make(chan struct{})
	go func() { q.Run(ctx); close(done) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	if processed != 3 {
		t.Fatalf("expected 3 processed, got %d", processed)
	}
}

func TestQueue_Full(t *testing.T) {
	bs, reg, ch, mgr := newPipeline(t)
	w := &Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr}
	q := NewQueue(w, 1)
	if err := q.Submit(Task{KBName: "docs"}); err != nil {
		t.Fatal(err)
	}
	if err := q.Submit(Task{KBName: "docs"}); err == nil {
		t.Fatal("expected queue-full error")
	}
}
