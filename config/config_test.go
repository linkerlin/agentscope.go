package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")
	raw := `{
		"name": "test",
		"system_prompt": "sys",
		"max_iterations": 10,
		"model": {"provider": "openai", "model_name": "gpt-4o-mini"},
		"memory": {"type": "window", "max_messages": 100, "max_tokens": 8000},
		"reme": {"enabled": true, "working_dir": "/tmp/reme", "max_input_length": 64000},
		"toolkit": {"parallel": true, "max_parallel": 4, "timeout_ms": 30000, "max_retries": 2}
	}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "test" || c.ReMe == nil || !c.ReMe.Enabled || c.ReMe.MaxInputLength != 64000 {
		t.Fatalf("%+v", c.ReMe)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	_, err := LoadFromFile(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFromYAMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	raw := `
name: test-yaml
system_prompt: sys
max_iterations: 5
model:
  provider: openai
  model_name: gpt-4o-mini
memory:
  type: window
  max_messages: 50
reme:
  enabled: true
  working_dir: /tmp/reme
  max_input_length: 32000
toolkit:
  parallel: true
  max_parallel: 2
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "test-yaml" || c.MaxIterations != 5 {
		t.Fatalf("unexpected config: %+v", c)
	}
	if c.ReMe == nil || !c.ReMe.Enabled || c.ReMe.MaxInputLength != 32000 {
		t.Fatalf("unexpected reme: %+v", c.ReMe)
	}
	if c.Memory.MaxMessages != 50 {
		t.Fatalf("unexpected memory: %+v", c.Memory)
	}
}

func TestLoadFromYMLExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yml")
	raw := `name: yml-test`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "yml-test" {
		t.Fatalf("expected yml-test, got %s", c.Name)
	}
}

func TestAgentConfigJSONRoundTrip(t *testing.T) {
	c := &AgentConfig{
		Name: "a",
		ReMe: &ReMeMemoryConfig{Enabled: true, WorkingDir: "w"},
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var out AgentConfig
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.ReMe == nil || out.ReMe.WorkingDir != "w" {
		t.Fatal(out.ReMe)
	}
}
