package skill

import (
	"fmt"

	"github.com/linkerlin/agentscope.go/toolkit"
)

const defaultSkillGroup = "basic"

// AttachOptions configures RegisterWithToolkit.
type AttachOptions struct {
	// SkillDirs loads skills from filesystem directories (each subdir is a skill).
	SkillDirs []string
	// AutoActivate marks loaded filesystem skills as active.
	AutoActivate bool
	// GroupName is the tool group for SkillViewer (default "basic").
	GroupName string
	// RegisterViewer controls SkillViewer registration (default true when skills exist).
	RegisterViewer *bool
}

// RegisterWithToolkit loads skills, registers SkillViewer on tk when skills exist,
// and returns a Hook that injects skill instructions before model calls.
// If reg is nil a new Registry is created.
func RegisterWithToolkit(tk *toolkit.Toolkit, reg *Registry, opts AttachOptions) (*Registry, *Hook, error) {
	if tk == nil {
		return nil, nil, fmt.Errorf("skill: toolkit is nil")
	}
	if reg == nil {
		reg = NewRegistry()
	}

	for _, dir := range opts.SkillDirs {
		repo := NewFileSystemRepository(dir)
		skills, err := repo.GetAllSkills()
		if err != nil {
			return reg, nil, fmt.Errorf("skill: load from %s: %w", dir, err)
		}
		for _, s := range skills {
			reg.Register(s)
			if opts.AutoActivate {
				reg.SetActive(s.SkillID(), true)
			}
		}
	}

	provider := NewPromptProvider(reg)
	skillHook := NewHook(provider)

	if len(reg.List()) == 0 {
		return reg, skillHook, nil
	}

	registerViewer := true
	if opts.RegisterViewer != nil {
		registerViewer = *opts.RegisterViewer
	}
	if !registerViewer {
		return reg, skillHook, nil
	}

	group := opts.GroupName
	if group == "" {
		group = defaultSkillGroup
	}
	if !tk.Groups.HasGroup(group) {
		if err := tk.Groups.CreateGroup(group, "Agent skills"); err != nil {
			return reg, skillHook, err
		}
	}

	viewer := NewSkillViewerTool(reg)
	if _, exists := tk.Registry.Get(viewer.Name()); !exists {
		if err := tk.Register(viewer); err != nil {
			return reg, skillHook, err
		}
	}
	_ = tk.Groups.AddTool(group, viewer.Name())
	_ = tk.Groups.SetGroupActive(group, true)

	return reg, skillHook, nil
}

// GetSkillInstructions returns the skill system prompt or empty string.
func GetSkillInstructions(reg *Registry) string {
	if reg == nil {
		return ""
	}
	return NewPromptProvider(reg).GetSkillPrompt()
}
