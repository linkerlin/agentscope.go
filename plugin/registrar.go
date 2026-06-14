package plugin

import (
	"fmt"
)

// Convenience methods for plugins to register items through the Registrar.
// These methods track what was registered and invoke the appropriate factory/registrar.

// RegisterTool is called by plugins to register a named tool.
// The framework's tool registration callback must be set via AddToolRegistrar.
func (r *Registrar) RegisterTool(name string, tool any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.toolRegistrars) == 0 {
		return fmt.Errorf("no tool registrar configured")
	}

	for _, fn := range r.toolRegistrars {
		if err := fn(func(n string, t any) error { return nil }); err != nil {
			return err
		}
	}
	r.registeredTools = append(r.registeredTools, name)
	return nil
}

// RegisterHook is called by plugins to register a named hook.
func (r *Registrar) RegisterHook(name string, hook any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.hookRegistrars) == 0 {
		return fmt.Errorf("no hook registrar configured")
	}

	for _, fn := range r.hookRegistrars {
		if err := fn(func(n string, h any) error { return nil }); err != nil {
			return err
		}
	}
	r.registeredHooks = append(r.registeredHooks, name)
	return nil
}

// RegisterMiddleware is called by plugins to add middleware.
func (r *Registrar) RegisterMiddleware(name string, mw any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.middlewareAdders) == 0 {
		return fmt.Errorf("no middleware adder configured")
	}

	for _, fn := range r.middlewareAdders {
		if err := fn(func(n string, m any) error { return nil }); err != nil {
			return err
		}
	}
	r.registeredMiddleware = append(r.registeredMiddleware, name)
	return nil
}

// CreateModel creates a model instance using the registered factory.
func (r *Registrar) CreateModel(typeName string, params map[string]any) (any, error) {
	r.mu.Lock()
	factory, ok := r.modelFactories[typeName]
	r.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no model factory registered for type %q", typeName)
	}

	inst, err := factory(params)
	if err != nil {
		return nil, fmt.Errorf("create model %q: %w", typeName, err)
	}

	r.recordRegistration("model", typeName)
	return inst, nil
}

// CreateFormatter creates a formatter instance using the registered factory.
func (r *Registrar) CreateFormatter(typeName string, params map[string]any) (any, error) {
	r.mu.Lock()
	factory, ok := r.formatterFactories[typeName]
	r.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no formatter factory registered for type %q", typeName)
	}

	inst, err := factory(params)
	if err != nil {
		return nil, fmt.Errorf("create formatter %q: %w", typeName, err)
	}

	r.recordRegistration("formatter", typeName)
	return inst, nil
}

// CreateMemory creates a memory backend instance using the registered factory.
func (r *Registrar) CreateMemory(typeName string, params map[string]any) (any, error) {
	r.mu.Lock()
	factory, ok := r.memoryFactories[typeName]
	r.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no memory factory registered for type %q", typeName)
	}

	inst, err := factory(params)
	if err != nil {
		return nil, fmt.Errorf("create memory %q: %w", typeName, err)
	}

	r.recordRegistration("memory", typeName)
	return inst, nil
}

// AvailableModelTypes returns the names of all registered model factories.
func (r *Registrar) AvailableModelTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.modelFactories))
	for k := range r.modelFactories {
		out = append(out, k)
	}
	return out
}

// AvailableFormatterTypes returns the names of all registered formatter factories.
func (r *Registrar) AvailableFormatterTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.formatterFactories))
	for k := range r.formatterFactories {
		out = append(out, k)
	}
	return out
}

// AvailableMemoryTypes returns the names of all registered memory factories.
func (r *Registrar) AvailableMemoryTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.memoryFactories))
	for k := range r.memoryFactories {
		out = append(out, k)
	}
	return out
}
