// scheduled_report demonstrates scheduling an Agent to run periodically.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/schedule"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	model, err := openai.Builder().APIKey(apiKey).ModelName("gpt-4o-mini").Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("Reporter").
		SysPrompt("You are a daily standup assistant. Summarize the given topic in one sentence.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	scheduler := schedule.NewScheduler(func(ctx context.Context, job *schedule.Job) error {
		resp, err := agent.Call(ctx, message.NewMsg().
			Role(message.RoleUser).
			TextContent(job.Payload).
			Build())
		if err != nil {
			return err
		}
		fmt.Printf("[%s] Job %s result: %s\n", time.Now().Format(time.RFC3339), job.ID, resp.GetTextContent())
		return nil
	})

	scheduler.Start()
	defer scheduler.Stop()

	ctx := context.Background()
	if err := scheduler.Schedule(ctx, &schedule.Job{
		ID:       "daily-report",
		AgentID:  "reporter",
		CronExpr: "*/10 * * * * *", // every 10 seconds for demo
		Payload:  "Generate a one-sentence status update about the AgentScope.Go project.",
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Scheduled. Waiting for 3 runs...")
	time.Sleep(35 * time.Second)
	fmt.Println("Done.")
}
