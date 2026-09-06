package finetune

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

func TestTrainingDataset_ExportSFTAndJSONL(t *testing.T) {
	d := NewTrainingDataset()
	d.AddSample(&TrainingSample{
		ID: "s1",
		Messages: []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
			message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(),
		},
		Reward:    1,
		Feedback:  "ok",
		Source:    "human",
		CreatedAt: time.Now(),
	})
	sft := d.ExportSFT()
	if len(sft) != 1 {
		t.Fatalf("sft len=%d", len(sft))
	}
	msgs, ok := sft[0]["messages"].([]map[string]string)
	if !ok || len(msgs) != 2 || msgs[0]["role"] != "user" {
		t.Fatalf("sft messages=%v", sft[0]["messages"])
	}
	raw, err := d.ExportJSONL()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("jsonl=%q", raw)
	}
	var round TrainingSample
	if err := json.Unmarshal(raw[:len(raw)-1], &round); err != nil {
		t.Fatal(err)
	}
	if round.ID != "s1" {
		t.Fatalf("roundtrip id=%s", round.ID)
	}
}

func TestTrainingDataset_ExportRLPreferencePairs(t *testing.T) {
	d := NewTrainingDataset()
	d.AddSample(&TrainingSample{
		Messages: []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent("q").Build(),
			message.NewMsg().Role(message.RoleAssistant).TextContent("good").Build(),
		},
		Reward:   2,
		Metadata: map[string]any{"task_id": "t1"},
	})
	d.AddSample(&TrainingSample{
		Messages: []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent("q").Build(),
			message.NewMsg().Role(message.RoleAssistant).TextContent("bad").Build(),
		},
		Reward:   0,
		Metadata: map[string]any{"task_id": "t1"},
	})
	pairs := d.ExportRL()
	if len(pairs) != 1 {
		t.Fatalf("pairs=%d", len(pairs))
	}
	if pairs[0]["chosen"] != "good" || pairs[0]["rejected"] != "bad" {
		t.Fatalf("pair=%v", pairs[0])
	}
}

func TestEventCollector_CollectsOnReplyEnd(t *testing.T) {
	d := NewTrainingDataset()
	c := NewEventCollector(d)
	ch := make(chan event.AgentEvent, 4)
	ch <- event.NewTextBlockDelta("r1", 0, "hi")
	ch <- event.NewReplyEnd("r1", "agent")
	close(ch)
	c.Collect(context.Background(), ch)
	if len(d.samples) != 1 {
		t.Fatalf("samples=%d", len(d.samples))
	}
	if d.samples[0].Source != "agent_run" {
		t.Fatalf("source=%s", d.samples[0].Source)
	}
}

func TestAutoEvaluator_ScoresExpectedReply(t *testing.T) {
	ev := NewAutoEvaluator()
	msg := message.NewMsg().Role(message.RoleAssistant).
		TextContent("The capital of France is Paris and it is a well known fact.").Build()
	score := ev.Evaluate(msg, "The capital of France is Paris")
	if score <= 0 {
		t.Fatalf("score=%v", score)
	}
	empty := ev.Evaluate(message.NewMsg().Role(message.RoleAssistant).TextContent("").Build(), "x")
	if empty >= score {
		t.Fatalf("empty score %v >= %v", empty, score)
	}
}
