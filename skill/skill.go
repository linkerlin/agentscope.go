package skill

import "github.com/linkerlin/agentscope.go/evolver"

// AgentSkill 表示一个可被 Agent 加载和使用的技能
type AgentSkill struct {
	Name         string
	Description  string
	SkillContent string
	Resources    map[string]string
	Source       string
}

// DistillToGene 将 ad-hoc Skill 蒸馏为 GEP Gene（对齐 evolver skill2gep / skillDistiller 优势）。
// 详见 evolver.DistillSkillToGene 及 gep 文档。
func (s *AgentSkill) DistillToGene(category string) evolver.Gene {
	if s == nil {
		return evolver.Gene{}
	}
	// 委托 evolver 包实现（包含 signals 提取、strategy 启发式、source 标记）。
	// 生产中可替换为更强的 LLM distiller。
	return evolver.DistillSkillToGene(evolver.AgentSkill{
		Name:         s.Name,
		Description:  s.Description,
		SkillContent: s.SkillContent,
		Resources:    s.Resources,
		Source:       s.Source,
	}, category)
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
