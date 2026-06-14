package plugin

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/toolkit"
)

// ExampleToolPlugin demonstrates a complete plugin that registers a tool.
// This serves as a reference implementation for plugin authors.
type ExampleToolPlugin struct {
	toolName string
	enabled  bool
}

func (p *ExampleToolPlugin) Name() string { return "example-tool-plugin" }

func (p *ExampleToolPlugin) Init(cfg PluginConfig) error {
	name, ok := cfg.Params["tool_name"].(string)
	if !ok {
		name = "echo"
	}
	p.toolName = name
	p.enabled = true
	return nil
}

func (p *ExampleToolPlugin) Register(r *Registrar) error {
	if !p.enabled {
		return nil
	}
	r.recordRegistration("tool", p.toolName)
	return nil
}

func (p *ExampleToolPlugin) Shutdown() error { return nil }

// TestExamplePluginLifecycle demonstrates the full plugin lifecycle.
func TestExamplePluginLifecycle(t *testing.T) {
	m := NewManager()
	m.Add(&ExampleToolPlugin{})

	r := NewRegistrar()

	if err := m.InitAll(); err != nil {
		t.Fatalf("InitAll: %v", err)
	}
	if err := m.RegisterAll(r); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	tools := r.RegisteredTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 registered tool, got %d: %v", len(tools), tools)
	}
	if tools[0] != "echo" {
		t.Errorf("expected 'echo', got %q", tools[0])
	}

	_ = m.ShutdownAll(context.Background())
}

// TestPluginConfigWithParams verifies that plugin config params are correctly
// passed through the full lifecycle.
func TestPluginConfigWithParams(t *testing.T) {
	m := NewManager()
	m.AddWithConfig(&ExampleToolPlugin{}, PluginConfig{
		Name:    "example-tool-plugin",
		Type:    "example",
		Enabled: true,
		Params:  map[string]any{"tool_name": "custom-echo"},
	})

	r := NewRegistrar()
	_ = m.InitAll()
	_ = m.RegisterAll(r)

	tools := r.RegisteredTools()
	if len(tools) != 1 || tools[0] != "custom-echo" {
		t.Errorf("expected ['custom-echo'], got %v", tools)
	}
}

// TestPluginWithToolkitIntegration shows how the Registrar bridges
// with the actual toolkit.Registry.
func TestPluginWithToolkitIntegration(t *testing.T) {
	// Create a real toolkit
	tk := toolkit.NewToolkit()
	_ = tk

	// Create registrar with tool registration callback
	r := NewRegistrar()
	r.AddToolRegistrar(func(registerTool func(name string, tool any) error) error {
		// In real usage, this would call tk.Register(tool.(tool.Tool))
		// Here we just validate the mechanism works
		return nil
	})

	m := NewManager()
	m.AddWithConfig(&ExampleToolPlugin{}, PluginConfig{
		Name:    "example",
		Enabled: true,
		Params:  map[string]any{"tool_name": "demo-tool"},
	})

	_ = m.InitAll()
	_ = m.RegisterAll(r)

	tools := r.RegisteredTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
}

// TestMultiplePluginsPriorityOrder verifies that plugins are initialized
// and shut down in correct priority order.
func TestMultiplePluginsPriorityOrder(t *testing.T) {
	var log []string
	var mu sync.Mutex

	record := func(s string) {
		mu.Lock()
		log = append(log, s)
		mu.Unlock()
	}

	m := NewManager()

	// Add in reverse priority order
	m.AddWithConfig(&mockPlugin{
		name:      "gamma",
		initFn:    func(c PluginConfig) error { record("init:gamma"); return nil },
		shutdownF: func() error { record("stop:gamma"); return nil },
	}, PluginConfig{Name: "gamma", Enabled: true, Priority: 300})

	m.AddWithConfig(&mockPlugin{
		name:      "alpha",
		initFn:    func(c PluginConfig) error { record("init:alpha"); return nil },
		shutdownF: func() error { record("stop:alpha"); return nil },
	}, PluginConfig{Name: "alpha", Enabled: true, Priority: 100})

	m.AddWithConfig(&mockPlugin{
		name:      "beta",
		initFn:    func(c PluginConfig) error { record("init:beta"); return nil },
		shutdownF: func() error { record("stop:beta"); return nil },
	}, PluginConfig{Name: "beta", Enabled: true, Priority: 200})

	_ = m.InitAll()
	_ = m.RegisterAll(NewRegistrar())
	_ = m.ShutdownAll(context.Background())

	// Verify init order: alpha, beta, gamma (by priority)
	expected := []string{"init:alpha", "init:beta", "init:gamma", "stop:gamma", "stop:beta", "stop:alpha"}
	if len(log) != len(expected) {
		t.Fatalf("expected %d events, got %d: %v", len(expected), len(log), log)
	}
	for i, e := range expected {
		if log[i] != e {
			t.Errorf("event[%d]: expected %q, got %q (full: %v)", i, e, log[i], log)
		}
	}
}

func TestPluginConfigDisabled(t *testing.T) {
	m := NewManager()
	cfg := Config{
		Plugins: []PluginConfig{
			{Name: "p1", Type: "type1", Enabled: false},
		},
	}
	if err := m.LoadConfig(cfg); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("disabled plugin should not be loaded, got count=%d", m.Count())
	}
}

func TestRegistrarMemoryFactory(t *testing.T) {
	r := NewRegistrar()
	r.SetMemoryFactory("custom-memory", func(params map[string]any) (any, error) {
		return fmt.Sprintf("memory-%v", params["name"]), nil
	})

	inst, err := r.CreateMemory("custom-memory", map[string]any{"name": "test"})
	if err != nil {
		t.Fatalf("CreateMemory: %v", err)
	}
	s, ok := inst.(string)
	if !ok {
		t.Fatalf("expected string, got %T", inst)
	}
	if s != "memory-test" {
		t.Errorf("expected 'memory-test', got %q", s)
	}

	if len(r.RegisteredMemories()) != 1 {
		t.Errorf("expected 1 registered memory, got %d", len(r.RegisteredMemories()))
	}
}

func TestRegistrarFormatterFactory(t *testing.T) {
	r := NewRegistrar()
	r.SetFormatterFactory("custom-fmt", func(params map[string]any) (any, error) {
		return map[string]string{"formatter": "custom"}, nil
	})

	inst, err := r.CreateFormatter("custom-fmt", nil)
	if err != nil {
		t.Fatalf("CreateFormatter: %v", err)
	}
	m := inst.(map[string]string)
	if m["formatter"] != "custom" {
		t.Errorf("expected 'custom', got %q", m["formatter"])
	}

	if len(r.RegisteredFormatters()) != 1 {
		t.Errorf("expected 1 registered formatter, got %d", len(r.RegisteredFormatters()))
	}
}
