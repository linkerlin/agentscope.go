package toolkit

import (
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// Registry 工具名到 Tool 的线程安全注册表
type Registry struct {
	mu    sync.RWMutex
	tools map[string]tool.Tool
}

// NewRegistry 创建空注册表
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]tool.Tool)}
}

// Register 注册工具；同名已存在时返回错误
func (r *Registry) Register(t tool.Tool) error {
	if t == nil {
		return fmt.Errorf("toolkit: nil tool")
	}
	name := t.Name()
	if name == "" {
		return fmt.Errorf("toolkit: empty tool name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[name]; ok {
		return fmt.Errorf("toolkit: tool already registered: %s", name)
	}
	r.tools[name] = t
	return nil
}

// MustRegister 同 Register，失败则 panic（仅用于初始化）
func (r *Registry) MustRegister(t tool.Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// Get 按名称获取工具
func (r *Registry) Get(name string) (tool.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 返回全部已注册工具（副本）
func (r *Registry) List() []tool.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]tool.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// ToolSpecs 生成模型可用的工具模式列表
func (r *Registry) ToolSpecs() []model.ToolSpec {
	ts := r.List()
	specs := make([]model.ToolSpec, 0, len(ts))
	for _, t := range ts {
		specs = append(specs, t.Spec())
	}
	return specs
}
