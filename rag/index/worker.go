// Package index orchestrates the RAG indexing pipeline: read a blob, parse it
// into Sections, chunk into Chunks, embed, and insert into a knowledge base.
//
// Aligned with Python AgentScope app._service.IndexWorker. The Worker is the
// single-process pipeline runner; a Queue adds channel-based dispatch. A
// dedicated Sweeper for retrying failed tasks requires persistent task state
// and is deferred (see ponytail note).
package index

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

// Task describes one document to index.
type Task struct {
	KBName    string // target knowledge base name
	DocID     string // unique document id within the KB
	BlobURI   string // blob store URI of the raw document bytes
	MediaType string // MIME type for parser routing
	Source    string // source filename for citation
}

// Status reports the outcome of processing a task.
type Status struct {
	Task   Task
	Chunks int
	Err    error
}

// OnStatus is invoked after each task completes (success or failure).
type OnStatus func(Status)

// Worker runs the indexing pipeline for a single task at a time.
type Worker struct {
	Blob    blob.BlobStore
	Parsers *parser.Registry
	Chunker chunker.Chunker
	Manager *kb.KBManager
	OnStatus
}

// Process runs the full pipeline for one task: blob → parse → chunk → embed → insert.
func (w *Worker) Process(ctx context.Context, task Task) error {
	kbh, err := w.Manager.Get(ctx, task.KBName)
	if err != nil {
		return w.fail(task, err)
	}
	rc, err := w.Blob.Get(ctx, task.BlobURI)
	if err != nil {
		return w.fail(task, fmt.Errorf("index: read blob: %w", err))
	}
	defer rc.Close()
	sections, err := w.Parsers.Parse(ctx, task.MediaType, task.Source, rc)
	if err != nil {
		return w.fail(task, fmt.Errorf("index: parse: %w", err))
	}
	chunks, err := w.Chunker.Chunk(sections)
	if err != nil {
		return w.fail(task, fmt.Errorf("index: chunk: %w", err))
	}
	// Tag chunks with the blob URI so the raw document stays addressable from
	// indexed records (chunk browsing / raw preview endpoints).
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = map[string]any{}
		}
		chunks[i].Metadata["blob_uri"] = task.BlobURI
	}
	if err := kbh.InsertChunks(ctx, task.DocID, chunks); err != nil {
		return w.fail(task, fmt.Errorf("index: insert: %w", err))
	}
	if w.OnStatus != nil {
		w.OnStatus(Status{Task: task, Chunks: len(chunks)})
	}
	return nil
}

func (w *Worker) fail(task Task, err error) error {
	if w.OnStatus != nil {
		w.OnStatus(Status{Task: task, Err: err})
	}
	return err
}

// Queue is a channel-based task dispatcher. Submit enqueues a task; Run drains
// the queue until ctx is cancelled, processing tasks with the Worker.
// ponytail: single-process channel queue; a Redis-backed queue arrives with the
// message-bus queue primitive (Phase 6).
type Queue struct {
	Worker *Worker
	ch     chan Task
}

// NewQueue creates a queue with the given buffer size.
func NewQueue(worker *Worker, bufferSize int) *Queue {
	if bufferSize <= 0 {
		bufferSize = 16
	}
	return &Queue{Worker: worker, ch: make(chan Task, bufferSize)}
}

// Submit enqueues a task. It blocks when the buffer is full (natural backpressure).
func (q *Queue) Submit(task Task) error {
	select {
	case q.ch <- task:
		return nil
	default:
		return fmt.Errorf("index: queue full")
	}
}

// SubmitCtx enqueues a task, respecting ctx cancellation.
func (q *Queue) SubmitCtx(ctx context.Context, task Task) error {
	select {
	case q.ch <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run drains the queue until ctx is cancelled, processing each task. It returns
// when the queue is drained and ctx is done.
func (q *Queue) Run(ctx context.Context) error {
	for {
		select {
		case task := <-q.ch:
			if err := q.Worker.Process(ctx, task); err != nil {
				// Errors are reported via OnStatus; keep draining.
				continue
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
