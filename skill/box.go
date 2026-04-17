package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	"github.com/linkerlin/agentscope.go/tool/shell"
	"github.com/linkerlin/agentscope.go/toolkit"
)

// Box manages registered skills, prompt injection, and code execution tooling.
type Box struct {
	registry    *Registry
	provider    *PromptProvider
	tk          *toolkit.Toolkit
	workDir     string
	uploadDir   string
	fileFilter  FileFilter
	autoUpload  bool
	loadTool    tool.Tool
}

// NewBox creates a SkillBox with an optional toolkit binding.
func NewBox(tk *toolkit.Toolkit) *Box {
	r := NewRegistry()
	return &Box{
		registry:   r,
		provider:   NewPromptProvider(r),
		tk:         tk,
		autoUpload: true,
	}
}

// Register registers a skill in the box.
func (b *Box) Register(s *AgentSkill) {
	b.registry.Register(s)
}

// GetSkill returns a skill by its skill ID.
func (b *Box) GetSkill(skillID string) (*AgentSkill, bool) {
	return b.registry.Get(skillID)
}

// DeactivateAllSkills sets all registered skills to inactive.
func (b *Box) DeactivateAllSkills() {
	b.registry.SetAllActive(false)
}

// GetSkillPrompt returns the system prompt containing all registered skills.
func (b *Box) GetSkillPrompt() string {
	return b.provider.GetSkillPrompt()
}

// BindToolkit rebinds the toolkit (useful when the toolkit is cloned by an agent).
func (b *Box) BindToolkit(tk *toolkit.Toolkit) {
	if tk == nil {
		panic("skill: toolkit cannot be nil")
	}
	b.tk = tk
}

// CodeExecution returns a builder for configuring code execution tools.
func (b *Box) CodeExecution() *CodeExecutionBuilder {
	return &CodeExecutionBuilder{box: b}
}

// SetAutoUpload controls whether skill files are automatically uploaded.
func (b *Box) SetAutoUpload(auto bool) {
	b.autoUpload = auto
}

// UploadSkillFiles writes skill resources to the upload directory.
func (b *Box) UploadSkillFiles() error {
	targetDir := b.ensureUploadDir()
	filter := b.fileFilter
	if filter == nil {
		filter = AcceptAllFilter()
	}
	for _, s := range b.registry.List() {
		paths := s.ResourcePaths()
		if len(paths) == 0 {
			continue
		}
		skillDir := filepath.Join(targetDir, s.SkillID())
		for _, p := range paths {
			if !filter(p) {
				continue
			}
			content := s.Resource(p)
			if content == "" {
				continue
			}
			targetPath := filepath.Join(skillDir, p)
			targetPath, _ = filepath.Abs(targetPath)
			skillDirAbs, _ := filepath.Abs(skillDir)
			if !strings.HasPrefix(targetPath, skillDirAbs) {
				continue // path traversal protection
			}
			_ = os.MkdirAll(filepath.Dir(targetPath), 0o755)
			_ = os.WriteFile(targetPath, []byte(content), 0o644)
		}
	}
	return nil
}

func (b *Box) ensureUploadDir() string {
	if b.uploadDir != "" {
		_ = os.MkdirAll(b.uploadDir, 0o755)
		return b.uploadDir
	}
	if b.workDir != "" {
		d := filepath.Join(b.workDir, "skills")
		_ = os.MkdirAll(d, 0o755)
		return d
	}
	d, _ := os.MkdirTemp("", "agentscope-skills-")
	b.uploadDir = d
	return d
}

// RegisterSkillLoadTool registers the load_skill_through_path tool to the toolkit.
func (b *Box) RegisterSkillLoadTool() error {
	if b.tk == nil {
		return fmt.Errorf("skill: toolkit not bound")
	}
	if !b.tk.Groups.HasGroup("skill_load_tools") {
		if err := b.tk.Groups.CreateGroup("skill_load_tools", "Skill loading tools"); err != nil {
			return err
		}
	}
	lt := newLoadSkillTool(b.registry)
	if err := b.tk.Registry.Register(lt); err != nil {
		return err
	}
	_ = b.tk.Groups.AddTool("skill_load_tools", lt.Name())
	_ = b.tk.Groups.SetGroupActive("skill_load_tools", true)
	return nil
}

// CodeExecutionBuilder configures and enables code execution tools.
type CodeExecutionBuilder struct {
	box              *Box
	workDir          string
	uploadDir        string
	customFilter     FileFilter
	includeFolders   []string
	includeExtensions []string
	customShellTool  *shell.ShellCommandTool
	withShellCalled  bool
	enableRead       bool
	enableWrite      bool
	codeExecPrompt   string
}

// WorkDir sets the working directory for code execution.
func (cb *CodeExecutionBuilder) WorkDir(dir string) *CodeExecutionBuilder {
	cb.workDir = dir
	return cb
}

// UploadDir sets the upload directory for skill files.
func (cb *CodeExecutionBuilder) UploadDir(dir string) *CodeExecutionBuilder {
	cb.uploadDir = dir
	return cb
}

// FileFilter sets a custom upload filter.
func (cb *CodeExecutionBuilder) FileFilter(f FileFilter) *CodeExecutionBuilder {
	cb.customFilter = f
	return cb
}

// IncludeFolders sets folders to include for uploads.
func (cb *CodeExecutionBuilder) IncludeFolders(folders ...string) *CodeExecutionBuilder {
	cb.includeFolders = folders
	return cb
}

// IncludeExtensions sets file extensions to include for uploads.
func (cb *CodeExecutionBuilder) IncludeExtensions(exts ...string) *CodeExecutionBuilder {
	cb.includeExtensions = exts
	return cb
}

// WithShell enables the shell command tool with default allowed commands.
func (cb *CodeExecutionBuilder) WithShell() *CodeExecutionBuilder {
	cb.withShellCalled = true
	cb.customShellTool = nil
	return cb
}

// WithCustomShell enables the shell command tool with a custom configuration.
func (cb *CodeExecutionBuilder) WithCustomShell(t *shell.ShellCommandTool) *CodeExecutionBuilder {
	if t == nil {
		panic("skill: ShellCommandTool cannot be nil")
	}
	cb.withShellCalled = true
	cb.customShellTool = t
	return cb
}

// WithRead enables the read file tool.
func (cb *CodeExecutionBuilder) WithRead() *CodeExecutionBuilder {
	cb.enableRead = true
	return cb
}

// WithWrite enables the write file tool.
func (cb *CodeExecutionBuilder) WithWrite() *CodeExecutionBuilder {
	cb.enableWrite = true
	return cb
}

// CodeExecutionInstruction sets a custom instruction appended to the skill prompt.
func (cb *CodeExecutionBuilder) CodeExecutionInstruction(inst string) *CodeExecutionBuilder {
	cb.codeExecPrompt = inst
	return cb
}

// Enable applies the configuration and registers the selected tools.
func (cb *CodeExecutionBuilder) Enable() error {
	if cb.box.tk == nil {
		return fmt.Errorf("skill: must bind toolkit before enabling code execution")
	}
	if cb.customFilter != nil && (len(cb.includeFolders) > 0 || len(cb.includeExtensions) > 0) {
		return fmt.Errorf("skill: cannot use FileFilter with IncludeFolders or IncludeExtensions")
	}

	groupName := "skill_code_execution_tool_group"
	if cb.box.tk.Groups.HasGroup(groupName) {
		_ = cb.box.tk.Groups.RemoveGroup(groupName)
	}

	cb.box.workDir = cb.workDir
	if cb.uploadDir != "" {
		cb.box.uploadDir = cb.uploadDir
	} else if cb.box.workDir != "" {
		cb.box.uploadDir = filepath.Join(cb.box.workDir, "skills")
	}

	if cb.customFilter != nil {
		cb.box.fileFilter = cb.customFilter
	} else {
		folders := cb.includeFolders
		if len(folders) == 0 {
			folders = []string{"scripts/", "assets/"}
		}
		exts := cb.includeExtensions
		if len(exts) == 0 {
			exts = []string{".py", ".js", ".sh"}
		}
		cb.box.fileFilter = DefaultFileFilter(folders, exts)
	}

	if !cb.box.tk.Groups.HasGroup(groupName) {
		if err := cb.box.tk.Groups.CreateGroup(groupName, "Code execution tools for skills"); err != nil {
			return err
		}
	}

	shellEnabled := false
	if cb.withShellCalled {
		var shellTool *shell.ShellCommandTool
		if cb.customShellTool != nil {
			shellTool = cloneShellTool(cb.customShellTool, cb.box.workDir)
		} else {
			shellTool = shell.NewShellCommandTool(cb.box.workDir, []string{"python", "python3", "node"}, nil)
		}
		if err := cb.box.tk.Registry.Register(shellTool); err != nil {
			return err
		}
		_ = cb.box.tk.Groups.AddTool(groupName, shellTool.Name())
		shellEnabled = true
	}

	if cb.enableRead {
		rt := file.NewReadFileTool(cb.box.workDir)
		if err := cb.box.tk.Registry.Register(rt); err != nil {
			return err
		}
		_ = cb.box.tk.Groups.AddTool(groupName, rt.Name())
	}

	if cb.enableWrite {
		wt := file.NewWriteFileTool(cb.box.workDir)
		if err := cb.box.tk.Registry.Register(wt); err != nil {
			return err
		}
		_ = cb.box.tk.Groups.AddTool(groupName, wt.Name())
	}

	_ = cb.box.tk.Groups.SetGroupActive(groupName, true)

	injectPrompt := shellEnabled || cb.codeExecPrompt != ""
	cb.box.provider.Instruction = DefaultSkillInstruction
	if injectPrompt {
		uploadDir := cb.box.ensureUploadDir()
		inst := cb.codeExecPrompt
		if inst == "" {
			inst = defaultCodeExecutionInstruction(uploadDir)
		}
		cb.box.provider.Instruction = DefaultSkillInstruction + "\n" + inst
	}
	return nil
}

func cloneShellTool(src *shell.ShellCommandTool, workDir string) *shell.ShellCommandTool {
	allowed := make([]string, 0, len(src.AllowedCommands))
	for c := range src.AllowedCommands {
		allowed = append(allowed, c)
	}
	t := shell.NewShellCommandTool(workDir, allowed, src.ApprovalCallback)
	if src.Validator != nil {
		t.Validator = src.Validator
	}
	if src.DefaultTimeout > 0 {
		t.DefaultTimeout = src.DefaultTimeout
	}
	return t
}

func defaultCodeExecutionInstruction(uploadDir string) string {
	return fmt.Sprintf(`## Code Execution

You have access to the execute_shell_command tool. When a task can be accomplished by running a pre-deployed skill script, you MUST execute it yourself using execute_shell_command rather than describing or suggesting commands to the user.

Skills root directory: %s
Each skill's files are located under a subdirectory named by its <skill-id>:
  %s/<skill-id>/scripts/
  %s/<skill-id>/assets/

Rules:
- Always use absolute paths when executing scripts
- If a script exists for the task, run it directly — do not rewrite its logic inline
- If asset/data files exist for the task, read them directly — do not recreate them
`, uploadDir, uploadDir, uploadDir)
}
