package skill

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// SkillViewerTool lets the agent browse skill markdown by name (PyV2 SkillViewer).
type SkillViewerTool struct {
	registry *Registry
}

// NewSkillViewerTool creates a SkillViewer bound to a skill registry.
func NewSkillViewerTool(registry *Registry) *SkillViewerTool {
	return &SkillViewerTool{registry: registry}
}

func (t *SkillViewerTool) Name() string { return "Skill" }

func (t *SkillViewerTool) Description() string {
	return "Retrieve a skill within the conversation. Check available skills when users ask you to perform specialized tasks."
}

func (t *SkillViewerTool) Spec() model.ToolSpec {
	enum := []string{}
	if t.registry != nil {
		for _, s := range t.registry.List() {
			if t.registry.IsActive(s.SkillID()) {
				enum = append(enum, s.Name)
			}
		}
	}
	skillProp := map[string]any{
		"type":        "string",
		"description": "The exact name of the skill to view.",
	}
	if len(enum) > 0 {
		skillProp["enum"] = enum
	}
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill": skillProp,
			},
			"required": []string{"skill"},
		},
	}
}

func (t *SkillViewerTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	name, _ := input["skill"].(string)
	if name == "" {
		return tool.NewTextResponse("SkillNotFoundError: skill name is required"), nil
	}
	if t.registry == nil {
		return tool.NewTextResponse(fmt.Sprintf("SkillNotFoundError: Skill '%s' not found.", name)), nil
	}
	for _, s := range t.registry.List() {
		if !t.registry.IsActive(s.SkillID()) {
			continue
		}
		if s.Name == name {
			content := s.SkillContent
			if content == "" {
				content = "(empty skill content)"
			}
			return tool.NewTextResponse(content), nil
		}
	}
	return tool.NewTextResponse(fmt.Sprintf("SkillNotFoundError: Skill '%s' not found.", name)), nil
}

func (t *SkillViewerTool) IsReadOnly() bool { return true }

func (t *SkillViewerTool) CheckPermissions(_ map[string]any, _ any) (tool.PermissionDecision, string, string, bool) {
	return tool.PermAllow, "The skill viewer is always allowed to be called.", "skill viewer", false
}

func (t *SkillViewerTool) MatchRule(pattern string, _ map[string]any) bool {
	return pattern == ""
}

func (t *SkillViewerTool) GenerateSuggestions(_ map[string]any) []tool.SuggestedRule {
	return []tool.SuggestedRule{{
		Name:     "suggested-tool-level",
		ToolName: t.Name(),
		Target:   "tool_name",
		Pattern:  t.Name(),
		Decision: tool.PermAllow,
	}}
}
