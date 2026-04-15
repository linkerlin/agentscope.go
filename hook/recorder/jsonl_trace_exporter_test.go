package recorder

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

func TestJsonlTraceExporter_BasicEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	exporter, err := NewBuilder(path).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	defer exporter.Close()

	agent := "TestAgent"
	now := time.Now()

	// PreCall
	_, _ = exporter.OnStreamEvent(nil, &hook.PreReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPreCall, Ts: now, Agent: agent},
		Messages:  []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()},
		ModelName: "gpt-4o",
	})

	// PreReasoning
	_, _ = exporter.OnStreamEvent(nil, &hook.PreReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPreReasoning, Ts: now, Agent: agent},
		Messages:  []*message.Msg{},
		ModelName: "gpt-4o",
	})

	// PostReasoning
	_, _ = exporter.OnStreamEvent(nil, &hook.PostReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: now, Agent: agent},
		Response:  message.NewMsg().Role(message.RoleAssistant).TextContent("hi").Build(),
	})

	// PreActing
	_, _ = exporter.OnStreamEvent(nil, &hook.PreActingEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPreActing, Ts: now, Agent: agent},
		ToolName:  "calc",
		ToolInput: map[string]any{"a": 1},
	})

	// PostActing
	_, _ = exporter.OnStreamEvent(nil, &hook.PostActingEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPostActing, Ts: now, Agent: agent},
		ToolName:  "calc",
		Result:    42,
	})

	// Error
	_, _ = exporter.OnStreamEvent(nil, &hook.ErrorEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: now, Agent: agent},
		Err:       errors.New("something went wrong"),
	})

	// flush and read
	exporter.Close()

	lines := readLines(t, path)
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d", len(lines))
	}

	events := []string{}
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		ev, _ := rec["event_type"].(string)
		events = append(events, ev)

		if rec["agent_name"] != agent {
			t.Fatalf("expected agent_name %s, got %v", agent, rec["agent_name"])
		}
		if rec["run_id"] == "" {
			t.Fatal("expected non-empty run_id")
		}
	}

	expected := []string{"pre_call", "pre_reasoning", "post_reasoning", "pre_acting", "post_acting", "error"}
	for i, ev := range expected {
		if events[i] != ev {
			t.Fatalf("expected event %s at line %d, got %s", ev, i, events[i])
		}
	}

	// verify turn/step counters
	var rec0 map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &rec0)
	if rec0["turn_id"].(float64) != 1 {
		t.Fatalf("expected turn_id=1 for first pre_call, got %v", rec0["turn_id"])
	}

	var rec1 map[string]any
	_ = json.Unmarshal([]byte(lines[1]), &rec1)
	if rec1["turn_id"].(float64) != 1 {
		t.Fatalf("expected turn_id=1 for pre_reasoning, got %v", rec1["turn_id"])
	}
	if rec1["step_id"].(float64) != 1 {
		t.Fatalf("expected step_id=1 for first pre_reasoning, got %v", rec1["step_id"])
	}
}

func TestJsonlTraceExporter_ChunkFiltering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	exporter, err := NewBuilder(path).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	defer exporter.Close()

	agent := "TestAgent"
	now := time.Now()

	// ReasoningChunk should be filtered by default
	_, _ = exporter.OnStreamEvent(nil, &hook.ReasoningChunkEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventReasoningChunk, Ts: now, Agent: agent},
		Chunk:     "thinking...",
	})

	// ActingChunk should be filtered by default
	_, _ = exporter.OnStreamEvent(nil, &hook.ActingChunkEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventActingChunk, Ts: now, Agent: agent},
		Chunk:     "progress...",
	})

	exporter.Close()

	lines := readLines(t, path)
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines when chunks are filtered, got %d", len(lines))
	}
}

func TestJsonlTraceExporter_IncludeChunks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	exporter, err := NewBuilder(path).
		IncludeReasoningChunks(true).
		IncludeActingChunks(true).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	defer exporter.Close()

	agent := "TestAgent"
	now := time.Now()

	_, _ = exporter.OnStreamEvent(nil, &hook.ReasoningChunkEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventReasoningChunk, Ts: now, Agent: agent},
		Chunk:     "thinking...",
	})

	_, _ = exporter.OnStreamEvent(nil, &hook.ActingChunkEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventActingChunk, Ts: now, Agent: agent},
		Chunk:     "progress...",
	})

	exporter.Close()

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestJsonlTraceExporter_FailFast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	exporter, err := NewBuilder(path).FailFast(true).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// close underlying file to force write error
	exporter.mu.Lock()
	_ = exporter.file.Close()
	exporter.mu.Unlock()

	// best-effort exporter should swallow write error
	bestEffort, _ := NewBuilder(filepath.Join(dir, "best.jsonl")).Build()
	bestEffort.mu.Lock()
	_ = bestEffort.file.Close()
	bestEffort.mu.Unlock()

	_, err = bestEffort.OnStreamEvent(nil, &hook.ErrorEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: "A"},
		Err:       errors.New("boom"),
	})
	if err != nil {
		t.Fatalf("best-effort exporter should swallow error, got: %v", err)
	}

	// fail-fast exporter should return write error
	_, err = exporter.OnStreamEvent(nil, &hook.ErrorEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: "A"},
		Err:       errors.New("boom"),
	})
	if err == nil {
		t.Fatal("fail-fast exporter should return error")
	}
}

func TestJsonlTraceExporter_ErrorEventPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	exporter, _ := NewBuilder(path).Build()
	defer exporter.Close()

	_, _ = exporter.OnStreamEvent(nil, &hook.ErrorEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: "A"},
		Err:       errors.New("boom"),
	})
	exporter.Close()

	lines := readLines(t, path)
	var rec map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &rec)

	if rec["error_message"] != "boom" {
		t.Fatalf("expected error_message=boom, got %v", rec["error_message"])
	}
	if !strings.Contains(rec["error_class"].(string), "errors.errorString") {
		t.Fatalf("expected error_class to contain errors.errorString, got %v", rec["error_class"])
	}
}

func TestJsonlTraceExporter_PostActingPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	exporter, _ := NewBuilder(path).Build()
	defer exporter.Close()

	_, _ = exporter.OnStreamEvent(nil, &hook.PostActingEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPostActing, Ts: time.Now(), Agent: "A"},
		ToolName:  "adder",
		ToolInput: map[string]any{"x": 10},
		Result:    20,
		Err:       nil,
	})
	exporter.Close()

	lines := readLines(t, path)
	var rec map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &rec)

	if rec["tool_name"] != "adder" {
		t.Fatalf("expected tool_name=adder, got %v", rec["tool_name"])
	}
	if rec["result"].(float64) != 20 {
		t.Fatalf("expected result=20, got %v", rec["result"])
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open trace file: %v", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
