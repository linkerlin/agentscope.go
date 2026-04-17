package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// loadSkillTool implements the load_skill_through_path agent tool.
type loadSkillTool struct {
	registry *Registry
}

func newLoadSkillTool(r *Registry) tool.Tool {
	return &loadSkillTool{registry: r}
}

func (t *loadSkillTool) Name() string        { return "load_skill_through_path" }
func (t *loadSkillTool) Description() string { return "Load and activate a skill resource by its ID and resource path." }

func (t *loadSkillTool) Spec() model.ToolSpec {
	skills := t.registry.List()
	enum := make([]string, len(skills))
	for i, s := range skills {
		enum[i] = s.SkillID()
	}
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the skill",
					"enum":        enum,
				},
				"path": map[string]any{
					"type":        "string",
					"description": "The path to the resource file within the skill (e.g., 'SKILL.md')",
				},
			},
			"required": []string{"skill_id", "path"},
		},
	}
}

func (t *loadSkillTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	skillID, _ := input["skill_id"].(string)
	path, _ := input["path"].(string)
	if skillID == "" {
		return nil, fmt.Errorf("missing required parameter: skill_id")
	}
	if path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	s, ok := t.registry.Get(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	// Activate skill
	t.registry.SetActive(skillID, true)

	if strings.EqualFold(path, "SKILL.md") {
		return tool.NewTextResponse(buildSkillMarkdownResponse(s)), nil
	}

	content := s.Resource(path)
	if content == "" {
		return nil, fmt.Errorf("resource not found: '%s' in skill '%s'. Available resources:\n%s", path, skillID, buildAvailableResources(s))
	}
	return tool.NewTextResponse(buildResourceResponse(s, path, content)), nil
}

func buildSkillMarkdownResponse(s *AgentSkill) string {
	return fmt.Sprintf("Successfully loaded skill: %s\n\nName: %s\nDescription: %s\nSource: %s\n\nContent:\n---\n%s\n---\n", s.SkillID(), s.Name, s.Description, s.Source, s.SkillContent)
}

func buildResourceResponse(s *AgentSkill, path, content string) string {
	return fmt.Sprintf("Successfully loaded resource from skill: %s\nResource path: %s\n\nContent:\n---\n%s\n---\n", s.SkillID(), path, content)
}

func buildAvailableResources(s *AgentSkill) string {
	var sb strings.Builder
	sb.WriteString("1. SKILL.md\n")
	for i, p := range s.ResourcePaths() {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+2, p))
	}
	return sb.String()
}
