package skill

import (
	"fmt"
	"strings"
)

// DefaultSkillInstruction 默认的系统提示头部
const DefaultSkillInstruction = `## Available Skills

<usage>
Skills provide specialized capabilities and domain knowledge. Use them when they match your current task.

How to use skills:
- Load skill: load_skill_through_path(skillId="<skill-id>", path="SKILL.md")
- The skill will be activated and its documentation loaded with detailed instructions
- Additional resources (scripts, assets, references) can be loaded using the same tool with different paths
</usage>

<available_skills>

`

// DefaultSkillTemplate 默认的技能模板
const DefaultSkillTemplate = `<skill>
<name>%s</name>
<description>%s</description>
<skill-id>%s</skill-id>
</skill>

`

// PromptProvider 根据注册表生成 skill system prompt
type PromptProvider struct {
	Registry    *Registry
	Instruction string
	Template    string
}

// NewPromptProvider 创建默认配置的 PromptProvider
func NewPromptProvider(registry *Registry) *PromptProvider {
	return &PromptProvider{
		Registry:    registry,
		Instruction: DefaultSkillInstruction,
		Template:    DefaultSkillTemplate,
	}
}

// GetSkillPrompt 生成包含所有已注册技能的系统提示
func (p *PromptProvider) GetSkillPrompt() string {
	skills := p.Registry.List()
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(p.Instruction)
	for _, s := range skills {
		sb.WriteString(formatSkill(p.Template, s))
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

func formatSkill(tmpl string, s *AgentSkill) string {
	if tmpl == "" {
		tmpl = DefaultSkillTemplate
	}
	return fmt.Sprintf(tmpl, s.Name, s.Description, s.SkillID())
}
