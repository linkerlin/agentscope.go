package async

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPool_Basic(t *testing.T) {
	p := NewPool(2, 10)
	defer p.Shutdown(context.Background())

	id := p.Submit(func() (any, error) {
		return "hello", nil
	})

	// Wait for completion.
	for i := 0; i < 50; i++ {
		if st := p.Status(id); st == StatusCompleted || st == StatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	task, ok := p.Task(id)
	if !ok {
		t.Fatal("task not found")
	}
	if task.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", task.Status)
	}
	if task.Result != "hello" {
		t.Fatalf("unexpected result: %v", task.Result)
	}
}

func TestPool_Error(t *testing.T) {
	p := NewPool(1, 10)
	defer p.Shutdown(context.Background())

	id := p.Submit(func() (any, error) {
		return nil, errors.New("boom")
	})

	for i := 0; i < 50; i++ {
		if st := p.Status(id); st == StatusCompleted || st == StatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	task, _ := p.Task(id)
	if task.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", task.Status)
	}
	if task.Error == nil || !strings.Contains(task.Error.Error(), "boom") {
		t.Fatalf("unexpected error: %v", task.Error)
	}
}

func TestPool_Concurrency(t *testing.T) {
	p := NewPool(3, 10)
	defer p.Shutdown(context.Background())

	start := time.Now()
	var ids []string
	for i := 0; i < 3; i++ {
		ids = append(ids, p.Submit(func() (any, error) {
			time.Sleep(50 * time.Millisecond)
			return nil, nil
		}))
	}

	for _, id := range ids {
		for j := 0; j < 100; j++ {
			if st := p.Status(id); st == StatusCompleted || st == StatusFailed {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	elapsed := time.Since(start)
	if elapsed > 120*time.Millisecond {
		t.Fatalf("expected concurrent execution (~50ms), took %v", elapsed)
	}
}

func TestPool_QueueBackpressure(t *testing.T) {
	p := NewPool(1, 1)
	defer p.Shutdown(context.Background())

	// Block the single worker.
	p.Submit(func() (any, error) {
		time.Sleep(200 * time.Millisecond)
		return nil, nil
	})

	// This should still fit in the queue.
	id := p.Submit(func() (any, error) {
		return "queued", nil
	})

	for i := 0; i < 100; i++ {
		if st := p.Status(id); st == StatusCompleted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	task, _ := p.Task(id)
	if task.Status != StatusCompleted || task.Result != "queued" {
		t.Fatalf("unexpected task state: %+v", task)
	}
}

func TestPool_Shutdown(t *testing.T) {
	p := NewPool(2, 10)

	id := p.Submit(func() (any, error) {
		time.Sleep(50 * time.Millisecond)
		return "done", nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	task, _ := p.Task(id)
	if task.Status != StatusCompleted {
		t.Fatalf("expected completed after shutdown, got %s", task.Status)
	}
}

func TestPool_ShutdownTimeout(t *testing.T) {
	p := NewPool(1, 10)

	p.Submit(func() (any, error) {
		time.Sleep(500 * time.Millisecond)
		return nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.Shutdown(ctx); err != context.DeadlineExceeded {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestPool_TaskNotFound(t *testing.T) {
	p := NewPool(1, 10)
	defer p.Shutdown(context.Background())

	_, ok := p.Task("non-existent-id")
	if ok {
		t.Fatal("expected task not found")
	}
}

func TestPool_StatusNotFound(t *testing.T) {
	p := NewPool(1, 10)
	defer p.Shutdown(context.Background())

	st := p.Status("non-existent-id")
	if st != "" {
		t.Fatalf("expected empty status for unknown task, got %q", st)
	}
}

func TestPool_ShutdownTwice(t *testing.T) {
	p := NewPool(1, 10)
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown failed: %v", err)
	}
	// Second shutdown should be safe (no-op or same error)
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown failed: %v", err)
	}
}

func TestPool_ManyTasks(t *testing.T) {
	p := NewPool(4, 100)
	defer p.Shutdown(context.Background())

	var ids []string
	for i := 0; i < 20; i++ {
		id := p.Submit(func() (any, error) {
			return "ok", nil
		})
		ids = append(ids, id)
	}

	for _, id := range ids {
		for j := 0; j < 100; j++ {
			if st := p.Status(id); st == StatusCompleted || st == StatusFailed {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		task, ok := p.Task(id)
		if !ok || task.Status != StatusCompleted || task.Result != "ok" {
			t.Fatalf("unexpected task state: %+v", task)
		}
	}
}
