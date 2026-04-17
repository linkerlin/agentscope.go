package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

func TestParseMarkdownWithFrontmatter(t *testing.T) {
	md := `---
name: data_analysis
description: Analyze CSV data
---
# Data Analysis
Read the file and produce insights.
`
	parsed, err := ParseMarkdownWithFrontmatter(md)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Metadata["name"] != "data_analysis" {
		t.Fatalf("unexpected name: %s", parsed.Metadata["name"])
	}
	if parsed.Metadata["description"] != "Analyze CSV data" {
		t.Fatalf("unexpected description: %s", parsed.Metadata["description"])
	}
	if !strings.Contains(parsed.Content, "Read the file") {
		t.Fatalf("unexpected content: %s", parsed.Content)
	}
}

func TestParseMarkdownWithoutFrontmatter(t *testing.T) {
	md := "# Hello\nWorld"
	parsed, err := ParseMarkdownWithFrontmatter(md)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Metadata) != 0 {
		t.Fatalf("expected empty metadata")
	}
	if parsed.Content != md {
		t.Fatalf("unexpected content: %s", parsed.Content)
	}
}

func TestGenerateMarkdownWithFrontmatter(t *testing.T) {
	meta := map[string]string{"name": "x", "desc": "y"}
	content := "body"
	out := GenerateMarkdownWithFrontmatter(meta, content)
	if !strings.HasPrefix(out, "---\n") {
		t.Fatalf("expected frontmatter prefix")
	}
	if !strings.Contains(out, "body") {
		t.Fatalf("missing content")
	}
}

func TestFileSystemRepository(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test_skill")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test
description: a test skill
---
Content here`), 0o644)
	_ = os.WriteFile(filepath.Join(skillDir, "helper.txt"), []byte("helper data"), 0o644)

	repo := NewFileSystemRepository(dir)
	skill, err := repo.GetSkill("test_skill")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "test" {
		t.Fatalf("expected name 'test', got %s", skill.Name)
	}
	if skill.Resource("helper.txt") != "helper data" {
		t.Fatalf("unexpected resource: %s", skill.Resource("helper.txt"))
	}

	names, err := repo.GetAllSkillNames()
	if err != nil || len(names) != 1 {
		t.Fatalf("expected 1 skill name, got %v", names)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	s := &AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"}
	r.Register(s)

	got, ok := r.Get(s.SkillID())
	if !ok || got.Name != "s1" {
		t.Fatal("expected skill")
	}
	if r.IsActive(s.SkillID()) {
		t.Fatal("expected inactive by default")
	}
	r.SetActive(s.SkillID(), true)
	if !r.IsActive(s.SkillID()) {
		t.Fatal("expected active")
	}
	r.SetAllActive(false)
	if r.IsActive(s.SkillID()) {
		t.Fatal("expected inactive after SetAllActive")
	}
}

func TestPromptProvider(t *testing.T) {
	r := NewRegistry()
	p := NewPromptProvider(r)
	if p.GetSkillPrompt() != "" {
		t.Fatal("expected empty prompt for empty registry")
	}
	r.Register(&AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"})
	prompt := p.GetSkillPrompt()
	if !strings.Contains(prompt, "<name>s1</name>") {
		t.Fatalf("expected skill name in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "</available_skills>") {
		t.Fatal("expected closing tag")
	}
}

func TestHook_OnEvent(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"})
	p := NewPromptProvider(r)
	h := NewHook(p)

	ctx := context.Background()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	hCtx := &hook.HookContext{
		Point:    hook.HookBeforeModel,
		Messages: []*message.Msg{userMsg},
	}

	res, err := h.OnEvent(ctx, hCtx)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.InjectMessages) != 2 {
		t.Fatalf("expected 2 messages, got %v", res)
	}
	if res.InjectMessages[0].Role != message.RoleSystem {
		t.Fatal("expected system message first")
	}
	if !strings.Contains(res.InjectMessages[0].GetTextContent(), "<name>s1</name>") {
		t.Fatalf("expected skill prompt in system message")
	}
}

func TestHook_OnEvent_AppendToExistingSystem(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"})
	p := NewPromptProvider(r)
	h := NewHook(p)

	ctx := context.Background()
	sysMsg := message.NewMsg().Role(message.RoleSystem).TextContent("base prompt").Build()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	hCtx := &hook.HookContext{
		Point:    hook.HookBeforeModel,
		Messages: []*message.Msg{sysMsg, userMsg},
	}

	res, err := h.OnEvent(ctx, hCtx)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.InjectMessages) != 2 {
		t.Fatalf("expected 2 messages, got %v", res)
	}
	text := res.InjectMessages[0].GetTextContent()
	if !strings.Contains(text, "base prompt") || !strings.Contains(text, "<name>s1</name>") {
		t.Fatalf("expected merged system prompt, got: %s", text)
	}
}

func TestHook_IgnoresOtherPoints(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentSkill{Name: "s1", Description: "d1", SkillContent: "c1"})
	h := NewHook(NewPromptProvider(r))

	ctx := context.Background()
	hCtx := &hook.HookContext{Point: hook.HookAfterModel, Messages: []*message.Msg{}}
	res, err := h.OnEvent(ctx, hCtx)
	if err != nil || res != nil {
		t.Fatal("expected nil result for non-before-model hook")
	}
}
