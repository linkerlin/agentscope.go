package skill

// AgentSkill 表示一个可被 Agent 加载和使用的技能
type AgentSkill struct {
	Name         string
	Description  string
	SkillContent string
	Resources    map[string]string
	Source       string
}

// SkillID 返回唯一标识符：name_source
func (s *AgentSkill) SkillID() string {
	src := s.Source
	if src == "" {
		src = "custom"
	}
	return s.Name + "_" + src
}

// Resource 按路径获取资源内容
func (s *AgentSkill) Resource(path string) string {
	if s.Resources == nil {
		return ""
	}
	return s.Resources[path]
}

// ResourcePaths 返回所有资源路径
func (s *AgentSkill) ResourcePaths() []string {
	if s.Resources == nil {
		return nil
	}
	out := make([]string, 0, len(s.Resources))
	for k := range s.Resources {
		out = append(out, k)
	}
	return out
}
