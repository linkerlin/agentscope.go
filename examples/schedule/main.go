// examples/schedule/main.go
//
// Demo: Cron-based job scheduler for agent execution.
//
// This demo shows how to create a scheduler, add a cron job, and list
// scheduled jobs. The handler is a stub that prints the job payload.
//
// How to run:
//   cd examples/schedule && go run main.go

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/schedule"
)

func main() {
	ctx := context.Background()

	// 1. Create a scheduler with a job handler.
	sch := schedule.NewScheduler(func(ctx context.Context, job *schedule.Job) error {
		fmt.Printf("[handler] job=%s agent=%s payload=%q\n", job.ID, job.AgentID, job.Payload)
		return nil
	})

	// 2. Start the scheduler background worker.
	sch.Start()
	defer sch.Stop()

	// 3. Add a job that runs every minute (cron expression).
	job := &schedule.Job{
		ID:       "demo-job-1",
		UserID:   "user-1",
		AgentID:  "agent-1",
		CronExpr: "* * * * *", // every minute
		Payload:  "ping",
		Enabled:  true,
		Timeout:  30 * time.Second,
	}
	if err := sch.Schedule(ctx, job); err != nil {
		fmt.Println("schedule error:", err)
		return
	}
	fmt.Println("scheduled job", job.ID)

	// 4. List active jobs and show the next run time.
	for _, j := range sch.ListJobs() {
		next, _ := sch.NextRun(j.ID)
		fmt.Printf("job=%s cron=%s next=%s\n", j.ID, j.CronExpr, next.Format(time.RFC3339))
	}

	// Keep alive briefly so the scheduler can fire at least once.
	time.Sleep(70 * time.Second)
}
