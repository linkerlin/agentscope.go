package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/toolkit"
)

func TestRegisterWithToolkit_RegistersViewerAndHook(t *testing.T) {
	tk := toolkit.NewToolkit()
	reg := NewRegistry()
	reg.Register(&AgentSkill{Name: "demo", Description: "d", SkillContent: "body", Source: "test"})
	reg.SetActive("demo_test", true)

	_, hook, err := RegisterWithToolkit(tk, reg, AttachOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if hook == nil {
		t.Fatal("expected hook")
	}
	if _, ok := tk.Registry.Get("Skill"); !ok {
		t.Fatal("expected Skill viewer registered")
	}
	prompt := GetSkillInstructions(reg)
	if !strings.Contains(prompt, "<name>demo</name>") || !strings.Contains(prompt, "`Skill`") {
		t.Fatalf("unexpected prompt:\n%s", prompt)
	}
}

func TestRegisterWithToolkit_LoadsSkillDirs(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "sample")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: sample\ndescription: from disk\n---\n# Sample\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := toolkit.NewToolkit()
	reg, _, err := RegisterWithToolkit(tk, nil, AttachOptions{
		SkillDirs:    []string{dir},
		AutoActivate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range reg.List() {
		if s.Name == "sample" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected loaded skill")
	}
	if _, ok := tk.Registry.Get("Skill"); !ok {
		t.Fatal("expected Skill viewer registered")
	}
}
