package skill

import "sync"

// Registry 管理技能的注册与激活状态
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*AgentSkill
	active map[string]bool
}

// NewRegistry 创建空的技能注册表
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*AgentSkill),
		active: make(map[string]bool),
	}
}

// Register 注册技能（若已存在则覆盖）
func (r *Registry) Register(skill *AgentSkill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := skill.SkillID()
	r.skills[id] = skill
}

// SetActive 设置技能的激活状态
func (r *Registry) SetActive(skillID string, active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[skillID] = active
}

// SetAllActive 设置所有已注册技能的激活状态
func (r *Registry) SetAllActive(active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.active {
		r.active[id] = active
	}
}

// IsActive 查询技能是否处于激活状态
func (r *Registry) IsActive(skillID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active[skillID]
}

// Get 按 ID 获取技能
func (r *Registry) Get(skillID string) (*AgentSkill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[skillID]
	return s, ok
}

// List 返回所有已注册技能
func (r *Registry) List() []*AgentSkill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*AgentSkill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// Remove 移除技能
func (r *Registry) Remove(skillID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, skillID)
	delete(r.active, skillID)
}
