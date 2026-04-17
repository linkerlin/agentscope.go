package skill

import (
	"fmt"
	"os"
	"path/filepath"
)

// Repository 定义技能的存储接口
type Repository interface {
	GetSkill(name string) (*AgentSkill, error)
	GetAllSkillNames() ([]string, error)
	GetAllSkills() ([]*AgentSkill, error)
}

// FileSystemRepository 基于文件系统的技能仓库
type FileSystemRepository struct {
	BaseDir  string
	Source   string
	Writable bool
}

// NewFileSystemRepository 创建文件系统仓库
func NewFileSystemRepository(baseDir string) *FileSystemRepository {
	return &FileSystemRepository{
		BaseDir:  baseDir,
		Writable: true,
		Source:   "filesystem",
	}
}

// GetSkill 从目录加载技能（读取 SKILL.md 及同级资源）
func (r *FileSystemRepository) GetSkill(name string) (*AgentSkill, error) {
	skillDir := filepath.Join(r.BaseDir, name)
	skillMD := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, fmt.Errorf("skill %s not found: %w", name, err)
	}
	parsed, err := ParseMarkdownWithFrontmatter(string(data))
	if err != nil {
		return nil, err
	}
	skillName := parsed.Metadata["name"]
	if skillName == "" {
		skillName = name
	}
	description := parsed.Metadata["description"]
	if description == "" {
		description = name
	}

	skill := &AgentSkill{
		Name:         skillName,
		Description:  description,
		SkillContent: parsed.Content,
		Resources:    make(map[string]string),
		Source:       r.sourceFor(name),
	}

	_ = filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(skillDir, path)
		if rel == "SKILL.md" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		skill.Resources[rel] = string(content)
		return nil
	})

	return skill, nil
}

// GetAllSkillNames 列出所有技能目录名
func (r *FileSystemRepository) GetAllSkillNames() ([]string, error) {
	entries, err := os.ReadDir(r.BaseDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// GetAllSkills 加载全部技能
func (r *FileSystemRepository) GetAllSkills() ([]*AgentSkill, error) {
	names, err := r.GetAllSkillNames()
	if err != nil {
		return nil, err
	}
	var skills []*AgentSkill
	for _, name := range names {
		s, err := r.GetSkill(name)
		if err != nil {
			continue
		}
		skills = append(skills, s)
	}
	return skills, nil
}

func (r *FileSystemRepository) sourceFor(name string) string {
	if r.Source != "" {
		return r.Source
	}
	base := filepath.Base(r.BaseDir)
	dir := filepath.Dir(r.BaseDir)
	parent := filepath.Base(dir)
	if parent == "." || parent == "" {
		return base
	}
	return parent + "_" + base
}
