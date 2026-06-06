package toolkit

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// ResetToolsMetaTool is a meta-tool that allows the agent to dynamically
// switch which tool groups are active at runtime.
type ResetToolsMetaTool struct {
	gm *GroupManager
}

// NewResetToolsMetaTool creates the meta-tool bound to a GroupManager.
func NewResetToolsMetaTool(gm *GroupManager) *ResetToolsMetaTool {
	return &ResetToolsMetaTool{gm: gm}
}

// Name returns the tool name.
func (r *ResetToolsMetaTool) Name() string { return "reset_equipped_tools" }

// Description returns the tool description.
func (r *ResetToolsMetaTool) Description() string {
	return "Activate or deactivate tool groups by name. Pass a JSON array of group names to activate. All other groups are deactivated. Pass [] to deactivate all groups and use the full registry."
}

// Spec returns the JSON schema.
func (r *ResetToolsMetaTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        r.Name(),
		Description: r.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"groups": map[string]any{
					"type":        "array",
					"description": "List of group names to activate",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{"groups"},
		},
	}
}

// Execute runs the meta-tool.
func (r *ResetToolsMetaTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	if r.gm == nil {
		return nil, fmt.Errorf("reset_equipped_tools: no GroupManager configured")
	}

	rawGroups, ok := input["groups"].([]any)
	if !ok {
		return nil, fmt.Errorf("reset_equipped_tools: groups must be an array")
	}

	var groupNames []string
	for _, g := range rawGroups {
		if s, ok := g.(string); ok {
			groupNames = append(groupNames, s)
		}
	}

	// Deactivate all groups first
	for name := range r.gm.groups {
		_ = r.gm.SetGroupActive(name, false)
	}

	// Activate requested groups
	var activated []string
	var failed []string
	for _, name := range groupNames {
		if !r.gm.HasGroup(name) {
			failed = append(failed, name)
			continue
		}
		if err := r.gm.SetGroupActive(name, true); err != nil {
			failed = append(failed, name)
			continue
		}
		activated = append(activated, name)
	}

	var sb strings.Builder
	if len(activated) > 0 {
		fmt.Fprintf(&sb, "Activated groups: %s. ", strings.Join(activated, ", "))
	}
	if len(failed) > 0 {
		fmt.Fprintf(&sb, "Unknown groups: %s. ", strings.Join(failed, ", "))
	}
	if len(activated) == 0 && len(failed) == 0 {
		sb.WriteString("All groups deactivated; using full tool registry.")
	}

	return tool.NewTextResponse(sb.String()), nil
}
