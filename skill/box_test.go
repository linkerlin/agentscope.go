package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/tool/shell"
	"github.com/linkerlin/agentscope.go/toolkit"
)

func TestBox_RegisterAndGetSkill(t *testing.T) {
	b := NewBox(nil)
	s := &AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"}
	b.Register(s)
	got, ok := b.GetSkill(s.SkillID())
	if !ok || got.Name != "s1" {
		t.Fatal("expected skill")
	}
}

func TestBox_DeactivateAllSkills(t *testing.T) {
	b := NewBox(nil)
	s := &AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"}
	b.Register(s)
	b.registry.SetActive(s.SkillID(), true)
	b.DeactivateAllSkills()
	if b.registry.IsActive(s.SkillID()) {
		t.Fatal("expected inactive")
	}
}

func TestBox_UploadSkillFiles(t *testing.T) {
	dir := t.TempDir()
	b := NewBox(nil)
	s := &AgentSkill{
		Name:         "s1",
		Description:  "d1",
		SkillContent: "c1",
		Resources:    map[string]string{"scripts/run.py": "print(1)", "readme.txt": "hi"},
	}
	b.Register(s)
	b.uploadDir = dir
	b.fileFilter = DefaultFileFilter([]string{"scripts/"}, []string{})
	b.UploadSkillFiles()

	uploaded := filepath.Join(dir, s.SkillID(), "scripts", "run.py")
	if _, err := os.Stat(uploaded); os.IsNotExist(err) {
		t.Fatalf("expected uploaded file %s", uploaded)
	}
	notUploaded := filepath.Join(dir, s.SkillID(), "readme.txt")
	if _, err := os.Stat(notUploaded); err == nil {
		t.Fatal("expected readme.txt to be skipped by filter")
	}
}

func TestBox_RegisterSkillLoadTool(t *testing.T) {
	tk := toolkit.NewToolkit()
	b := NewBox(tk)
	s := &AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"}
	b.Register(s)
	if err := b.RegisterSkillLoadTool(); err != nil {
		t.Fatal(err)
	}
	if !tk.Groups.HasGroup("skill_load_tools") {
		t.Fatal("expected group to exist")
	}
}

func TestCodeExecutionBuilder_Enable(t *testing.T) {
	tk := toolkit.NewToolkit()
	b := NewBox(tk)
	b.Register(&AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"})
	if err := b.CodeExecution().WithShell().WithRead().WithWrite().Enable(); err != nil {
		t.Fatal(err)
	}
	if !tk.Groups.HasGroup("skill_code_execution_tool_group") {
		t.Fatal("expected code execution group")
	}
	active := tk.Groups.ActiveTools()
	if len(active) != 3 {
		t.Fatalf("expected 3 active tools, got %d", len(active))
	}
	prompt := b.GetSkillPrompt()
	if !strings.Contains(prompt, "Code Execution") {
		t.Fatalf("expected code execution instruction in prompt, got:\n%s", prompt)
	}
}

func TestCodeExecutionBuilder_EnableCustomShell(t *testing.T) {
	tk := toolkit.NewToolkit()
	b := NewBox(tk)
	customShell := shell.NewShellCommandTool("", []string{"go"}, nil)
	if err := b.CodeExecution().WithCustomShell(customShell).Enable(); err != nil {
		t.Fatal(err)
	}
	active := tk.Groups.ActiveTools()
	if len(active) != 1 {
		t.Fatalf("expected 1 active tool, got %d", len(active))
	}
}

func TestCodeExecutionBuilder_ReplaceExisting(t *testing.T) {
	tk := toolkit.NewToolkit()
	b := NewBox(tk)
	_ = b.CodeExecution().WithShell().Enable()
	_ = b.CodeExecution().WithRead().Enable()
	active := tk.Groups.ActiveTools()
	if len(active) != 1 {
		t.Fatalf("expected 1 active tool after replace, got %d", len(active))
	}
}

func TestCodeExecutionBuilder_InvalidFilterCombo(t *testing.T) {
	tk := toolkit.NewToolkit()
	b := NewBox(tk)
	err := b.CodeExecution().FileFilter(AcceptAllFilter()).IncludeFolders("x").Enable()
	if err == nil || !strings.Contains(err.Error(), "cannot use FileFilter") {
		t.Fatalf("expected filter combo error, got %v", err)
	}
}

func TestLoadSkillTool_Execute(t *testing.T) {
	r := NewRegistry()
	s := &AgentSkill{Name: "s1", Description: "d1", SkillContent: "content here", Resources: map[string]string{"helper.txt": "help"}}
	r.Register(s)
	lt := newLoadSkillTool(r)

	resp, err := lt.Execute(nil, map[string]any{"skill_id": s.SkillID(), "path": "SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "content here") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	if !r.IsActive(s.SkillID()) {
		t.Fatal("expected skill to be activated")
	}
}

func TestLoadSkillTool_ResourceNotFound(t *testing.T) {
	r := NewRegistry()
	s := &AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"}
	r.Register(s)
	lt := newLoadSkillTool(r)

	_, err := lt.Execute(nil, map[string]any{"skill_id": s.SkillID(), "path": "missing.txt"})
	if err == nil || !strings.Contains(err.Error(), "resource not found") {
		t.Fatalf("expected resource not found error, got %v", err)
	}
}
