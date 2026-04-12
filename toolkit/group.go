package toolkit

import (
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/tool"
)

// ToolGroup 工具逻辑分组（不含执行逻辑）
type ToolGroup struct {
	Name        string
	Description string
	ToolNames   []string
}

// GroupManager 管理分组与「当前激活」集合；未创建任何分组时等价于使用 Registry 中全部工具
type GroupManager struct {
	registry *Registry
	mu       sync.RWMutex
	groups   map[string]*ToolGroup
	// active[name]=true 表示该分组参与可见工具集；若 len(active)>0 则仅返回激活分组内的工具
	active map[string]bool
}

// NewGroupManager 创建分组管理器
func NewGroupManager(reg *Registry) *GroupManager {
	return &GroupManager{
		registry: reg,
		groups:   make(map[string]*ToolGroup),
		active:   make(map[string]bool),
	}
}

// CreateGroup 新建空分组
func (gm *GroupManager) CreateGroup(name, description string) error {
	if name == "" {
		return fmt.Errorf("toolkit: empty group name")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if _, ok := gm.groups[name]; ok {
		return fmt.Errorf("toolkit: group exists: %s", name)
	}
	gm.groups[name] = &ToolGroup{Name: name, Description: description}
	return nil
}

// AddTool 将已注册工具名加入分组
func (gm *GroupManager) AddTool(groupName, toolName string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[groupName]
	if !ok {
		return fmt.Errorf("toolkit: unknown group: %s", groupName)
	}
	if _, ok := gm.registry.Get(toolName); !ok {
		return fmt.Errorf("toolkit: unknown tool: %s", toolName)
	}
	g.ToolNames = append(g.ToolNames, toolName)
	return nil
}

// SetGroupActive 设置分组是否参与可见工具
func (gm *GroupManager) SetGroupActive(name string, active bool) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if _, ok := gm.groups[name]; !ok {
		return fmt.Errorf("toolkit: unknown group: %s", name)
	}
	gm.active[name] = active
	return nil
}

// ActiveTools 返回当前应对模型暴露的工具列表
func (gm *GroupManager) ActiveTools() []tool.Tool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	if len(gm.groups) == 0 {
		return gm.registry.List()
	}

	hasActive := false
	for _, on := range gm.active {
		if on {
			hasActive = true
			break
		}
	}
	if !hasActive {
		return gm.registry.List()
	}

	seen := make(map[string]struct{})
	var out []tool.Tool
	for gname, on := range gm.active {
		if !on {
			continue
		}
		g := gm.groups[gname]
		if g == nil {
			continue
		}
		for _, tn := range g.ToolNames {
			if _, ok := seen[tn]; ok {
				continue
			}
			if tt, ok := gm.registry.Get(tn); ok {
				seen[tn] = struct{}{}
				out = append(out, tt)
			}
		}
	}
	return out
}
