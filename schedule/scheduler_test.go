package schedule

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_ScheduleAndCancel(t *testing.T) {
	var mu sync.Mutex
	var invocations []string

	handle := func(ctx context.Context, job *Job) error {
		mu.Lock()
		defer mu.Unlock()
		invocations = append(invocations, job.ID)
		return nil
	}

	s := NewScheduler(handle)
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	job := &Job{ID: "j1", CronExpr: "@every 1s", Payload: "hello"}
	if err := s.Schedule(ctx, job); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	count := len(invocations)
	mu.Unlock()
	if count < 1 {
		t.Fatalf("expected at least 1 invocation, got %d", count)
	}

	if err := s.Cancel(ctx, "j1"); err != nil {
		t.Fatal(err)
	}

	if err := s.Cancel(ctx, "unknown"); err == nil {
		t.Fatal("expected error for unknown job")
	}
}

func TestScheduler_NextRun(t *testing.T) {
	handle := func(ctx context.Context, job *Job) error { return nil }
	s := NewScheduler(handle)
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	job := &Job{ID: "j2", CronExpr: "0 0 * * *"}
	if err := s.Schedule(ctx, job); err != nil {
		t.Fatal(err)
	}

	next, err := s.NextRun("j2")
	if err != nil {
		t.Fatal(err)
	}
	if next.IsZero() {
		t.Fatal("expected non-zero next run")
	}

	if _, err := s.NextRun("unknown"); err == nil {
		t.Fatal("expected error for unknown job")
	}
}

func TestScheduler_InvalidCron(t *testing.T) {
	handle := func(ctx context.Context, job *Job) error { return nil }
	s := NewScheduler(handle)

	ctx := context.Background()
	job := &Job{ID: "j3", CronExpr: "invalid"}
	if err := s.Schedule(ctx, job); err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestScheduler_EmptyCron(t *testing.T) {
	handle := func(ctx context.Context, job *Job) error { return nil }
	s := NewScheduler(handle)

	ctx := context.Background()
	job := &Job{ID: "j4", CronExpr: ""}
	if err := s.Schedule(ctx, job); err == nil {
		t.Fatal("expected error for empty cron expression")
	}
}

func TestScheduler_DuplicateID(t *testing.T) {
	var count int64
	handle := func(ctx context.Context, job *Job) error {
		atomic.AddInt64(&count, 1)
		return nil
	}
	s := NewScheduler(handle)
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	job1 := &Job{ID: "dup", CronExpr: "@every 1h"}
	if err := s.Schedule(ctx, job1); err != nil {
		t.Fatal(err)
	}
	next1, err := s.NextRun("dup")
	if err != nil {
		t.Fatal(err)
	}

	// Replace with a different schedule.
	job2 := &Job{ID: "dup", CronExpr: "@every 2h"}
	if err := s.Schedule(ctx, job2); err != nil {
		t.Fatal(err)
	}
	next2, err := s.NextRun("dup")
	if err != nil {
		t.Fatal(err)
	}

	// The next-run time must have changed, proving the old entry was removed.
	if next2.Equal(next1) {
		t.Fatalf("expected NextRun to change after reschedule, got same time: %v", next2)
	}

	// The long interval means handler should not fire during the test.
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt64(&count) != 0 {
		t.Fatalf("expected 0 invocations with long interval, got %d", atomic.LoadInt64(&count))
	}
}

func TestScheduler_HandlerErrorContinues(t *testing.T) {
	var count int64
	handle := func(ctx context.Context, job *Job) error {
		atomic.AddInt64(&count, 1)
		return fmt.Errorf("handler error")
	}
	s := NewScheduler(handle)
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	job := &Job{ID: "j5", CronExpr: "@every 1s"}
	if err := s.Schedule(ctx, job); err != nil {
		t.Fatal(err)
	}

	time.Sleep(2500 * time.Millisecond)

	c := atomic.LoadInt64(&count)
	if c < 2 {
		t.Fatalf("expected at least 2 invocations despite errors, got %d", c)
	}
}

func TestScheduler_StopBeforeStart(t *testing.T) {
	handle := func(ctx context.Context, job *Job) error { return nil }
	s := NewScheduler(handle)
	// Stop without Start should not panic.
	s.Stop()
}

func TestScheduler_ConcurrentScheduleCancel(t *testing.T) {
	handle := func(ctx context.Context, job *Job) error { return nil }
	s := NewScheduler(handle)
	s.Start()
	defer s.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			job := &Job{ID: fmt.Sprintf("j%d", idx), CronExpr: "@every 1h"}
			_ = s.Schedule(ctx, job)
			_ = s.Cancel(ctx, fmt.Sprintf("j%d", idx))
		}(i)
	}
	wg.Wait()
}
