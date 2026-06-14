package plugin

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// mockPlugin is a test plugin implementation.
type mockPlugin struct {
	name      string
	initFn    func(config PluginConfig) error
	regFn     func(r *Registrar) error
	shutdownF func() error
	mu        sync.Mutex
	initCount int
	regCount  int
	stopCount int
	lastCfg   PluginConfig
}

func (m *mockPlugin) Name() string { return m.name }

func (m *mockPlugin) Init(cfg PluginConfig) error {
	m.mu.Lock()
	m.initCount++
	m.lastCfg = cfg
	m.mu.Unlock()
	if m.initFn != nil {
		return m.initFn(cfg)
	}
	return nil
}

func (m *mockPlugin) Register(r *Registrar) error {
	m.mu.Lock()
	m.regCount++
	m.mu.Unlock()
	if m.regFn != nil {
		return m.regFn(r)
	}
	return nil
}

func (m *mockPlugin) Shutdown() error {
	m.mu.Lock()
	m.stopCount++
	m.mu.Unlock()
	if m.shutdownF != nil {
		return m.shutdownF()
	}
	return nil
}

func TestPluginInterface(t *testing.T) {
	var _ Plugin = (*mockPlugin)(nil)
}

func TestManagerAddAndCount(t *testing.T) {
	m := NewManager()
	if m.Count() != 0 {
		t.Fatalf("expected 0 plugins, got %d", m.Count())
	}

	m.Add(&mockPlugin{name: "a"})
	m.Add(&mockPlugin{name: "b"})
	if m.Count() != 2 {
		t.Fatalf("expected 2 plugins, got %d", m.Count())
	}
}

func TestManagerGet(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "test"}
	m.Add(p)

	got := m.Get("test")
	if got == nil {
		t.Fatal("expected to find plugin 'test'")
	}
	if got.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", got.Name())
	}

	if m.Get("nonexistent") != nil {
		t.Fatal("expected nil for nonexistent plugin")
	}
}

func TestManagerInitAll(t *testing.T) {
	m := NewManager()
	p1 := &mockPlugin{name: "p1"}
	p2 := &mockPlugin{name: "p2"}
	m.Add(p1)
	m.Add(p2)

	if err := m.InitAll(); err != nil {
		t.Fatalf("InitAll failed: %v", err)
	}
	if p1.initCount != 1 {
		t.Errorf("p1 init count: expected 1, got %d", p1.initCount)
	}
	if p2.initCount != 1 {
		t.Errorf("p2 init count: expected 1, got %d", p2.initCount)
	}

	// Double init should fail
	if err := m.InitAll(); err == nil {
		t.Error("expected error on double InitAll")
	}
}

func TestManagerInitAllError(t *testing.T) {
	m := NewManager()
	m.Add(&mockPlugin{name: "ok"})
	m.Add(&mockPlugin{
		name:   "bad",
		initFn: func(c PluginConfig) error { return fmt.Errorf("init failed") },
	})

	err := m.InitAll()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestManagerRegisterAll(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "test"}
	m.Add(p)

	if err := m.RegisterAll(NewRegistrar()); err == nil {
		t.Error("expected error when registering without init")
	}

	if err := m.InitAll(); err != nil {
		t.Fatalf("InitAll: %v", err)
	}

	r := NewRegistrar()
	if err := m.RegisterAll(r); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if p.regCount != 1 {
		t.Errorf("register count: expected 1, got %d", p.regCount)
	}
}

func TestManagerRegisterAllError(t *testing.T) {
	m := NewManager()
	m.Add(&mockPlugin{
		name: "bad",
		regFn: func(r *Registrar) error {
			return fmt.Errorf("register failed")
		},
	})
	_ = m.InitAll()

	err := m.RegisterAll(NewRegistrar())
	if err == nil {
		t.Fatal("expected register error")
	}
}

func TestManagerShutdownAll(t *testing.T) {
	m := NewManager()
	p1 := &mockPlugin{name: "p1", shutdownF: func() error { return nil }}
	p2 := &mockPlugin{name: "p2", shutdownF: func() error { return nil }}
	m.Add(p1)
	m.Add(p2)

	_ = m.InitAll()
	_ = m.RegisterAll(NewRegistrar())

	if err := m.ShutdownAll(context.Background()); err != nil {
		t.Fatalf("ShutdownAll: %v", err)
	}
	if p1.stopCount != 1 {
		t.Errorf("p1 stop count: expected 1, got %d", p1.stopCount)
	}
	if p2.stopCount != 1 {
		t.Errorf("p2 stop count: expected 1, got %d", p2.stopCount)
	}
}

func TestManagerShutdownAllContinuesOnError(t *testing.T) {
	m := NewManager()
	p1 := &mockPlugin{name: "p1", shutdownF: func() error { return fmt.Errorf("shutdown error") }}
	p2 := &mockPlugin{name: "p2"}
	m.Add(p1)
	m.Add(p2)

	_ = m.InitAll()
	_ = m.RegisterAll(NewRegistrar())

	// Should not fail even if one plugin fails to shutdown
	_ = m.ShutdownAll(context.Background())
	if p2.stopCount != 1 {
		t.Errorf("p2 should have been shut down despite p1 error, got %d", p2.stopCount)
	}
}

func TestManagerPriorityOrder(t *testing.T) {
	m := NewManager()
	var order []string
	var mu sync.Mutex

	m.AddWithConfig(&mockPlugin{
		name: "p3",
		initFn: func(c PluginConfig) error {
			mu.Lock()
			order = append(order, "p3")
			mu.Unlock()
			return nil
		},
	}, PluginConfig{Name: "p3", Priority: 300, Enabled: true})

	m.AddWithConfig(&mockPlugin{
		name: "p1",
		initFn: func(c PluginConfig) error {
			mu.Lock()
			order = append(order, "p1")
			mu.Unlock()
			return nil
		},
	}, PluginConfig{Name: "p1", Priority: 100, Enabled: true})

	m.AddWithConfig(&mockPlugin{
		name: "p2",
		initFn: func(c PluginConfig) error {
			mu.Lock()
			order = append(order, "p2")
			mu.Unlock()
			return nil
		},
	}, PluginConfig{Name: "p2", Priority: 200, Enabled: true})

	if err := m.InitAll(); err != nil {
		t.Fatalf("InitAll: %v", err)
	}

	expected := []string{"p1", "p2", "p3"}
	if len(order) != 3 {
		t.Fatalf("expected 3 inits, got %d: %v", len(order), order)
	}
	for i, e := range expected {
		if order[i] != e {
			t.Errorf("init order[%d]: expected %q, got %q (full: %v)", i, e, order[i], order)
		}
	}
}

func TestManagerPlugins(t *testing.T) {
	m := NewManager()
	m.Add(&mockPlugin{name: "a"})
	m.Add(&mockPlugin{name: "b"})

	plugins := m.Plugins()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestRegisterFactory(t *testing.T) {
	m := NewManager()
	m.RegisterFactory("test-type", func() Plugin {
		return &mockPlugin{name: "test-instance"}
	})

	// Verify factory is stored by loading config
	cfg := Config{
		Plugins: []PluginConfig{
			{Name: "test", Type: "test-type", Enabled: true},
		},
	}
	if err := m.LoadConfig(cfg); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("expected 1 plugin, got %d", m.Count())
	}
}

func TestLoadConfigUnknownType(t *testing.T) {
	m := NewManager()
	cfg := Config{
		Plugins: []PluginConfig{
			{Name: "test", Type: "unknown", Enabled: true},
		},
	}
	if err := m.LoadConfig(cfg); err == nil {
		t.Error("expected error for unknown plugin type")
	}
}

func TestLoadConfigDisabledSkipped(t *testing.T) {
	m := NewManager()
	m.RegisterFactory("type1", func() Plugin { return &mockPlugin{name: "p"} })

	cfg := Config{
		Plugins: []PluginConfig{
			{Name: "p1", Type: "type1", Enabled: false},
			{Name: "p2", Type: "type1", Enabled: true},
		},
	}
	if err := m.LoadConfig(cfg); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("expected 1 plugin (disabled skipped), got %d", m.Count())
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     Config{},
			wantErr: false,
		},
		{
			name: "valid config",
			cfg: Config{
				Plugins: []PluginConfig{
					{Name: "a", Type: "type-a"},
					{Name: "b", Type: "type-b"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			cfg: Config{
				Plugins: []PluginConfig{
					{Name: "", Type: "type-a"},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate name",
			cfg: Config{
				Plugins: []PluginConfig{
					{Name: "a", Type: "type-a"},
					{Name: "a", Type: "type-b"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing type",
			cfg: Config{
				Plugins: []PluginConfig{
					{Name: "a", Type: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	yamlData := []byte(`
plugins:
  - name: tool-a
    type: tool
    enabled: true
    priority: 100
    params:
      api_key: "secret"
      max_retries: 3
  - name: model-b
    type: openai
    enabled: false
`)
	cfg, err := ParseConfig(yamlData)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Name != "tool-a" {
		t.Errorf("plugin[0] name: expected 'tool-a', got %q", cfg.Plugins[0].Name)
	}
	if !cfg.Plugins[0].Enabled {
		t.Error("plugin[0] should be enabled")
	}
	if cfg.Plugins[1].Enabled {
		t.Error("plugin[1] should be disabled")
	}
	if cfg.Plugins[0].Params["api_key"] != "secret" {
		t.Errorf("expected api_key='secret', got %v", cfg.Plugins[0].Params["api_key"])
	}
}

func TestEnabledPluginsSorted(t *testing.T) {
	cfg := Config{
		Plugins: []PluginConfig{
			{Name: "c", Type: "t", Enabled: true, Priority: 300},
			{Name: "a", Type: "t", Enabled: true, Priority: 100},
			{Name: "b", Type: "t", Enabled: false, Priority: 200},
			{Name: "d", Type: "t", Enabled: true, Priority: 200},
		},
	}
	enabled := cfg.EnabledPlugins()
	if len(enabled) != 3 {
		t.Fatalf("expected 3 enabled, got %d", len(enabled))
	}
	// Should be sorted: a(100), d(200), c(300)
	if enabled[0].Name != "a" {
		t.Errorf("expected 'a' first, got %q", enabled[0].Name)
	}
	if enabled[1].Name != "d" {
		t.Errorf("expected 'd' second, got %q", enabled[1].Name)
	}
	if enabled[2].Name != "c" {
		t.Errorf("expected 'c' third, got %q", enabled[2].Name)
	}
}

func TestRegistrarModelFactory(t *testing.T) {
	r := NewRegistrar()
	r.SetModelFactory("test-model", func(params map[string]any) (any, error) {
		return map[string]any{"type": "test", "params": params}, nil
	})

	inst, err := r.CreateModel("test-model", map[string]any{"key": "val"})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m := inst.(map[string]any)
	if m["type"] != "test" {
		t.Errorf("expected type 'test', got %v", m["type"])
	}

	if len(r.RegisteredModels()) != 1 {
		t.Errorf("expected 1 registered model, got %d", len(r.RegisteredModels()))
	}
}

func TestRegistrarUnknownModelType(t *testing.T) {
	r := NewRegistrar()
	_, err := r.CreateModel("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown model type")
	}
}

func TestRegistrarAvailableTypes(t *testing.T) {
	r := NewRegistrar()
	r.SetModelFactory("m1", func(p map[string]any) (any, error) { return nil, nil })
	r.SetModelFactory("m2", func(p map[string]any) (any, error) { return nil, nil })
	r.SetFormatterFactory("f1", func(p map[string]any) (any, error) { return nil, nil })

	if len(r.AvailableModelTypes()) != 2 {
		t.Errorf("expected 2 model types, got %d", len(r.AvailableModelTypes()))
	}
	if len(r.AvailableFormatterTypes()) != 1 {
		t.Errorf("expected 1 formatter type, got %d", len(r.AvailableFormatterTypes()))
	}
	if len(r.AvailableMemoryTypes()) != 0 {
		t.Errorf("expected 0 memory types, got %d", len(r.AvailableMemoryTypes()))
	}
}

func TestPluginStateString(t *testing.T) {
	tests := []struct {
		state PluginState
		want  string
	}{
		{StateRegistered, "registered"},
		{StateInitialized, "initialized"},
		{StateActive, "active"},
		{StateShutDown, "shut_down"},
		{StateError, "error"},
		{PluginState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("PluginState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestManagerFullLifecycle(t *testing.T) {
	m := NewManager()

	p1 := &mockPlugin{name: "tool-plugin"}
	p2 := &mockPlugin{name: "model-plugin"}

	m.Add(p1)
	m.Add(p2)

	// Init
	if err := m.InitAll(); err != nil {
		t.Fatalf("InitAll: %v", err)
	}

	// Register
	r := NewRegistrar()
	if err := m.RegisterAll(r); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	// Verify states
	for _, mp := range m.Plugins() {
		if mp.State != StateActive {
			t.Errorf("plugin %q state: expected active, got %s", mp.Plugin.Name(), mp.State)
		}
	}

	// Shutdown
	if err := m.ShutdownAll(context.Background()); err != nil {
		t.Fatalf("ShutdownAll: %v", err)
	}

	for _, mp := range m.Plugins() {
		if mp.State != StateShutDown {
			t.Errorf("plugin %q state: expected shut_down, got %s", mp.Plugin.Name(), mp.State)
		}
	}
}

func TestLoadSO_NonLinux(t *testing.T) {
	// On non-Linux platforms, this should return an error
	_, err := LoadSO("/path/to/plugin.so")
	if err == nil {
		t.Error("expected error on non-Linux platform")
	}
}
