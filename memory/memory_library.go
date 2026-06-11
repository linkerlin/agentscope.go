package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// MemoryLibrary 预构建记忆模板库
type MemoryLibrary struct {
	EntryPoint string
}

// LibraryEntry 库中的一条记忆条目
type LibraryEntry struct {
	MemoryType string   `json:"memory_type"`
	Target     string   `json:"target"`
	Content    string   `json:"content"`
	WhenToUse  string   `json:"when_to_use"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
}

// NewMemoryLibrary 创建记忆模板库
func NewMemoryLibrary(entryPoint string) *MemoryLibrary {
	return &MemoryLibrary{EntryPoint: entryPoint}
}

// LoadLibraryJSONFile 从 JSON 文件加载预构建记忆条目
func (lib *MemoryLibrary) LoadLibraryJSONFile(path string) ([]LibraryEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []LibraryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// LoadFromDir 从目录加载所有 JSON 记忆文件
func (lib *MemoryLibrary) LoadFromDir(dir string) ([]LibraryEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var allEntries []LibraryEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		loaded, err := lib.LoadLibraryJSONFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		allEntries = append(allEntries, loaded...)
	}
	return allEntries, nil
}

// InjectIntoVectorStore 将库中的记忆条目注入向量存储
func (lib *MemoryLibrary) InjectIntoVectorStore(ctx context.Context, store VectorStore, entries []LibraryEntry) error {
	nodes := make([]*MemoryNode, 0, len(entries))
	for _, entry := range entries {
		node := &MemoryNode{
			MemoryType:   MemoryType(entry.MemoryType),
			MemoryTarget: entry.Target,
			Content:      entry.Content,
			WhenToUse:    entry.WhenToUse,
			Metadata: map[string]any{
				"source":   "memory_library",
				"category": entry.Category,
				"tags":     entry.Tags,
			},
		}
		node.MemoryID = GenerateMemoryID(entry.Content + "|" + entry.Target)
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil
	}
	return store.Insert(ctx, nodes)
}

// GetDefaultAgentLibrary 返回 AI Agent 通用记忆模板。
// 包含常见的 Agent 行为模式、工具使用建议等。
func GetDefaultAgentLibrary() []LibraryEntry {
	return []LibraryEntry{
		{
			MemoryType: string(MemoryTypeTool),
			Target:     "web_search",
			Content:    "使用 site: 限定符缩小搜索范围，如在技术问题上使用 site:stackoverflow.com 可获得更精确的结果",
			WhenToUse:  "当使用 web_search 工具且需要精准技术答案时",
			Category:   "tool_best_practice",
			Tags:       []string{"web_search", "search_optimization"},
		},
		{
			MemoryType: string(MemoryTypeTool),
			Target:     "file_read",
			Content:    "读取大文件时应先检查文件大小，对于超过1MB的文件建议分块读取",
			WhenToUse:  "当准备读取文件且不确定文件大小时",
			Category:   "tool_best_practice",
			Tags:       []string{"file_read", "performance"},
		},
		{
			MemoryType: string(MemoryTypeProcedural),
			Target:     "debugging",
			Content:    "调试时遵循二分法：先缩小范围确定问题区域，再深入检查具体代码。先读错误日志，后读相关源码。",
			WhenToUse:  "当遇到代码错误或bug需要调试时",
			Category:   "task_strategy",
			Tags:       []string{"debugging", "systematic_approach"},
		},
		{
			MemoryType: string(MemoryTypeProcedural),
			Target:     "code_review",
			Content:    "代码审查优先关注：1) 正确性 2) 安全性（输入验证、注入防护） 3) 性能瓶颈 4) 可读性和命名",
			WhenToUse:  "当需要进行代码审查时",
			Category:   "task_strategy",
			Tags:       []string{"code_review", "best_practices"},
		},
		{
			MemoryType: string(MemoryTypeTool),
			Target:     "write_file",
			Content:    "写入文件前先确保父目录存在，使用 os.MkdirAll 或等效方法创建目录结构",
			WhenToUse:  "当使用写文件工具且目标路径可能不存在时",
			Category:   "tool_best_practice",
			Tags:       []string{"write_file", "file_system"},
		},
	}
}
