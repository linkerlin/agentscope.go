package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Plugin is the core interface that all plugins must implement.
type Plugin interface {
	// Name returns a unique identifier for this plugin.
	Name() string

	// Init initializes the plugin with the provided configuration.
	// Called once before Register.
	Init(config PluginConfig) error

	// Register adds the plugin's extension points to the framework.
	// Called once after Init succeeds.
	Register(r *Registrar) error

	// Shutdown cleans up resources held by the plugin.
	// Called once during graceful shutdown.
	Shutdown() error
}

// PluginConfig holds configuration for a plugin, loaded from YAML or set programmatically.
type PluginConfig struct {
	// Name is the plugin identifier (must match Plugin.Name()).
	Name string `yaml:"name" json:"name"`

	// Type identifies which plugin implementation to use (for factory-based loading).
	Type string `yaml:"type" json:"type"`

	// Enabled controls whether the plugin is active.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Priority controls initialization order (lower = earlier).
	Priority int `yaml:"priority" json:"priority"`

	// Params is plugin-specific configuration passed to Init.
	Params map[string]any `yaml:"params" json:"params"`
}

// PluginState tracks the lifecycle state of a managed plugin.
type PluginState int

const (
	StateRegistered PluginState = iota
	StateInitialized
	StateActive
	StateShutDown
	StateError
)

func (s PluginState) String() string {
	switch s {
	case StateRegistered:
		return "registered"
	case StateInitialized:
		return "initialized"
	case StateActive:
		return "active"
	case StateShutDown:
		return "shut_down"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ManagedPlugin wraps a Plugin with lifecycle metadata.
type ManagedPlugin struct {
	Plugin  Plugin
	Config  PluginConfig
	State   PluginState
	InitErr error
}

// Registrar provides typed registration methods for each extension point.
// Plugins use the Registrar during their Register() call to add their
// contributions to the framework.
type Registrar struct {
	mu sync.RWMutex

	// factories hold named constructors for each extension point type.
	// These are populated by the framework before plugins are registered.
	modelFactories     map[string]ModelFactory
	toolRegistrars     []ToolRegistrarFunc
	hookRegistrars     []HookRegistrarFunc
	middlewareAdders   []MiddlewareAddFunc
	formatterFactories map[string]FormatterFactory
	memoryFactories    map[string]MemoryFactory

	// Registered items (for inspection)
	registeredModels     []string
	registeredTools      []string
	registeredHooks      []string
	registeredMiddleware []string
	registeredFormatters []string
	registeredMemories   []string
}

// NewRegistrar creates a new empty Registrar.
func NewRegistrar() *Registrar {
	return &Registrar{
		modelFactories:     make(map[string]ModelFactory),
		formatterFactories: make(map[string]FormatterFactory),
		memoryFactories:    make(map[string]MemoryFactory),
	}
}

// SetModelFactory registers a model factory under a given type name.
func (r *Registrar) SetModelFactory(typeName string, factory ModelFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelFactories[typeName] = factory
}

// SetFormatterFactory registers a formatter factory under a given type name.
func (r *Registrar) SetFormatterFactory(typeName string, factory FormatterFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.formatterFactories[typeName] = factory
}

// SetMemoryFactory registers a memory factory under a given type name.
func (r *Registrar) SetMemoryFactory(typeName string, factory MemoryFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memoryFactories[typeName] = factory
}

// AddToolRegistrar adds a function that can register tools into a toolkit.
func (r *Registrar) AddToolRegistrar(fn ToolRegistrarFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toolRegistrars = append(r.toolRegistrars, fn)
}

// AddHookRegistrar adds a function that can register hooks.
func (r *Registrar) AddHookRegistrar(fn HookRegistrarFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hookRegistrars = append(r.hookRegistrars, fn)
}

// AddMiddlewareAdder adds a function that can add middleware.
func (r *Registrar) AddMiddlewareAdder(fn MiddlewareAddFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewareAdders = append(r.middlewareAdders, fn)
}

// RegisteredModels returns the names of models registered via this Registrar.
func (r *Registrar) RegisteredModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.registeredModels))
	copy(out, r.registeredModels)
	return out
}

// RegisteredTools returns the names of tools registered via this Registrar.
func (r *Registrar) RegisteredTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.registeredTools))
	copy(out, r.registeredTools)
	return out
}

// RegisteredHooks returns the names of hooks registered via this Registrar.
func (r *Registrar) RegisteredHooks() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.registeredHooks))
	copy(out, r.registeredHooks)
	return out
}

// RegisteredMiddleware returns the names of middleware registered via this Registrar.
func (r *Registrar) RegisteredMiddleware() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.registeredMiddleware))
	copy(out, r.registeredMiddleware)
	return out
}

// RegisteredFormatters returns the names of formatters registered via this Registrar.
func (r *Registrar) RegisteredFormatters() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.registeredFormatters))
	copy(out, r.registeredFormatters)
	return out
}

// RegisteredMemories returns the names of memory backends registered via this Registrar.
func (r *Registrar) RegisteredMemories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.registeredMemories))
	copy(out, r.registeredMemories)
	return out
}

// recordRegistration adds a name to the appropriate tracking slice.
func (r *Registrar) recordRegistration(kind, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch kind {
	case "model":
		r.registeredModels = append(r.registeredModels, name)
	case "tool":
		r.registeredTools = append(r.registeredTools, name)
	case "hook":
		r.registeredHooks = append(r.registeredHooks, name)
	case "middleware":
		r.registeredMiddleware = append(r.registeredMiddleware, name)
	case "formatter":
		r.registeredFormatters = append(r.registeredFormatters, name)
	case "memory":
		r.registeredMemories = append(r.registeredMemories, name)
	}
}

// Factory type definitions for each extension point.

// ModelFactory creates a model instance from parameters.
type ModelFactory func(params map[string]any) (any, error)

// FormatterFactory creates a formatter instance from parameters.
type FormatterFactory func(params map[string]any) (any, error)

// MemoryFactory creates a memory backend instance from parameters.
type MemoryFactory func(params map[string]any) (any, error)

// ToolRegistrarFunc is called to register tools into the framework's toolkit.
// The framework provides the toolkit registration function.
type ToolRegistrarFunc func(registerTool func(name string, tool any) error) error

// HookRegistrarFunc is called to register hooks.
type HookRegistrarFunc func(registerHook func(name string, hook any) error) error

// MiddlewareAddFunc is called to add middleware.
type MiddlewareAddFunc func(addMiddleware func(name string, mw any) error) error

// Manager orchestrates plugin lifecycle: loading, initialization, registration, and shutdown.
type Manager struct {
	mu        sync.Mutex
	plugins   []*ManagedPlugin
	state     ManagerState
	factories map[string]PluginFactory
}

// ManagerState tracks the overall state of the plugin manager.
type ManagerState int

const (
	ManagerCreated ManagerState = iota
	ManagerInitialized
	ManagerRegistered
	ManagerShutDown
)

// PluginFactory creates a Plugin instance by type name (for config-driven loading).
type PluginFactory func() Plugin

// NewManager creates a new plugin manager.
func NewManager() *Manager {
	return &Manager{
		factories: make(map[string]PluginFactory),
	}
}

// RegisterFactory registers a plugin factory by type name for config-driven loading.
func (m *Manager) RegisterFactory(typeName string, factory PluginFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factories[typeName] = factory
}

// Add adds a plugin instance directly (programmatic registration).
func (m *Manager) Add(p Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, &ManagedPlugin{
		Plugin: p,
		Config: PluginConfig{
			Name:    p.Name(),
			Enabled: true,
		},
		State: StateRegistered,
	})
}

// AddWithConfig adds a plugin instance with explicit config.
func (m *Manager) AddWithConfig(p Plugin, cfg PluginConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, &ManagedPlugin{
		Plugin: p,
		Config: cfg,
		State:  StateRegistered,
	})
}

// LoadConfig loads plugins from a Config struct.
// Plugins referenced by type name must have their factories pre-registered.
func (m *Manager) LoadConfig(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pc := range cfg.Plugins {
		if !pc.Enabled {
			continue
		}
		factory, ok := m.factories[pc.Type]
		if !ok {
			return fmt.Errorf("plugin %q: no factory registered for type %q", pc.Name, pc.Type)
		}
		p := factory()
		m.plugins = append(m.plugins, &ManagedPlugin{
			Plugin: p,
			Config: pc,
			State:  StateRegistered,
		})
	}
	return nil
}

// InitAll initializes all registered plugins in priority order.
func (m *Manager) InitAll() error {
	m.mu.Lock()
	if m.state >= ManagerInitialized {
		m.mu.Unlock()
		return fmt.Errorf("plugins already initialized")
	}
	// Sort in place so shutdown also follows the correct reverse order
	sortByPriority(m.plugins)
	plugins := make([]*ManagedPlugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.Unlock()

	for _, mp := range plugins {
		if mp.State != StateRegistered {
			continue
		}
		if err := mp.Plugin.Init(mp.Config); err != nil {
			mp.State = StateError
			mp.InitErr = err
			return fmt.Errorf("plugin %q init failed: %w", mp.Plugin.Name(), err)
		}
		mp.State = StateInitialized
	}

	m.mu.Lock()
	m.state = ManagerInitialized
	m.mu.Unlock()
	return nil
}

// RegisterAll registers all initialized plugins using the provided Registrar.
func (m *Manager) RegisterAll(r *Registrar) error {
	m.mu.Lock()
	if m.state < ManagerInitialized {
		m.mu.Unlock()
		return fmt.Errorf("plugins must be initialized before registration")
	}
	plugins := make([]*ManagedPlugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.Unlock()

	for _, mp := range plugins {
		if mp.State != StateInitialized {
			continue
		}
		if err := mp.Plugin.Register(r); err != nil {
			mp.State = StateError
			return fmt.Errorf("plugin %q register failed: %w", mp.Plugin.Name(), err)
		}
		mp.State = StateActive
	}

	m.mu.Lock()
	m.state = ManagerRegistered
	m.mu.Unlock()
	return nil
}

// ShutdownAll shuts down all active plugins in reverse priority order.
func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.mu.Lock()
	plugins := make([]*ManagedPlugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.Unlock()

	// Reverse order for shutdown
	for i := len(plugins) - 1; i >= 0; i-- {
		mp := plugins[i]
		if mp.State != StateActive {
			continue
		}
		if err := mp.Plugin.Shutdown(); err != nil {
			// Log but continue shutting down others
			mp.State = StateError
		} else {
			mp.State = StateShutDown
		}
	}

	m.mu.Lock()
	m.state = ManagerShutDown
	m.mu.Unlock()
	return nil
}

// Plugins returns information about all managed plugins.
func (m *Manager) Plugins() []ManagedPlugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ManagedPlugin, len(m.plugins))
	for i, mp := range m.plugins {
		out[i] = *mp
	}
	return out
}

// Get returns a plugin by name, or nil if not found.
func (m *Manager) Get(name string) Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mp := range m.plugins {
		if mp.Plugin.Name() == name {
			return mp.Plugin
		}
	}
	return nil
}

// Count returns the number of managed plugins.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.plugins)
}

func sortByPriority(plugins []*ManagedPlugin) {
	// Simple stable insertion sort by priority
	for i := 1; i < len(plugins); i++ {
		for j := i; j > 0 && plugins[j].Config.Priority < plugins[j-1].Config.Priority; j-- {
			plugins[j], plugins[j-1] = plugins[j-1], plugins[j]
		}
	}
}
