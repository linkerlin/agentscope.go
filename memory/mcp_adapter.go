package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/model"
)

// MCPMemoryToolSet 将 VectorMemory 的常见操作包装为可通过 MCP 协议调用的工具定义。
// 与 toolkit/mcp 配合使用：将这些工具注册到 MCP Manager 即可让远程 Agent 调用记忆操作。

// MCPMemoryToolSet 记忆 MCP 工具集
type MCPMemoryToolSet struct {
	Mem     VectorMemory
	ReMeMem *ReMeVectorMemory
}

// NewMCPMemoryToolSet 创建 MCP 工具集
func NewMCPMemoryToolSet(mem VectorMemory) *MCPMemoryToolSet {
	s := &MCPMemoryToolSet{Mem: mem}
	if r, ok := mem.(*ReMeVectorMemory); ok {
		s.ReMeMem = r
	}
	return s
}

// ListTools 返回所有记忆操作的工具规格列表
func (s *MCPMemoryToolSet) ListTools() []model.ToolSpec {
	return []model.ToolSpec{
		{
			Name:        "reme_search_memory",
			Description: "语义搜索记忆库，返回最相关的记忆条目。支持按类型（personal/procedural/tool/summary/history/identity）和目标过滤。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索查询文本",
					},
					"memory_type": map[string]any{
						"type":        "string",
						"description": "记忆类型过滤（personal/procedural/tool/summary/history/identity），可选",
					},
					"memory_target": map[string]any{
						"type":        "string",
						"description": "记忆目标过滤（用户名/任务名/工具名），可选",
					},
					"top_k": map[string]any{
						"type":    "integer",
						"default": 10,
					},
					"min_score": map[string]any{
						"type":    "number",
						"default": 0.1,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "reme_add_memory",
			Description: "向记忆库添加一条新记忆。记忆将自动嵌入向量并支持后续语义检索。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"description": "记忆内容",
					},
					"memory_type": map[string]any{
						"type":        "string",
						"enum":        []string{"personal", "procedural", "tool", "summary", "history", "identity"},
						"description": "记忆类型",
					},
					"memory_target": map[string]any{
						"type":        "string",
						"description": "记忆目标",
					},
					"when_to_use": map[string]any{
						"type":        "string",
						"description": "触发条件描述（用于精准检索，可选）",
					},
					"score": map[string]any{
						"type":    "number",
						"default": 1.0,
					},
				},
				"required": []string{"content", "memory_type"},
			},
		},
		{
			Name:        "reme_retrieve_personal",
			Description: "检索用户的个人偏好和画像记忆。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user_name": map[string]any{
						"type":        "string",
						"description": "用户名",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "查询内容",
					},
					"top_k": map[string]any{
						"type":    "integer",
						"default": 5,
					},
				},
				"required": []string{"user_name", "query"},
			},
		},
		{
			Name:        "reme_retrieve_procedural",
			Description: "检索任务执行经验和工作流程记忆。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_name": map[string]any{
						"type":        "string",
						"description": "任务名称",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "查询内容",
					},
					"top_k": map[string]any{
						"type":    "integer",
						"default": 5,
					},
				},
				"required": []string{"task_name", "query"},
			},
		},
		{
			Name:        "reme_retrieve_tool",
			Description: "检索工具使用经验和最佳实践记忆。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool_name": map[string]any{
						"type":        "string",
						"description": "工具名称",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "查询内容",
					},
					"top_k": map[string]any{
						"type":    "integer",
						"default": 5,
					},
				},
				"required": []string{"tool_name", "query"},
			},
		},
		{
			Name:        "reme_summarize",
			Description: "对对话历史进行记忆提取与持久化，自动生成个人/任务/工具三类记忆并去重存储。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user_name": map[string]any{
						"type":        "string",
						"description": "关联用户名（可选）",
					},
					"task_name": map[string]any{
						"type":        "string",
						"description": "关联任务名（可选）",
					},
					"tool_name": map[string]any{
						"type":        "string",
						"description": "关联工具名（可选）",
					},
				},
				"required": []string{},
			},
		},
	}
}

// Execute 根据工具名执行对应操作
func (s *MCPMemoryToolSet) Execute(ctx context.Context, name string, args map[string]any) (any, error) {
	switch name {
	case "reme_search_memory":
		return s.searchMemory(ctx, args)
	case "reme_add_memory":
		return s.addMemory(ctx, args)
	case "reme_retrieve_personal":
		return s.retrievePersonal(ctx, args)
	case "reme_retrieve_procedural":
		return s.retrieveProcedural(ctx, args)
	case "reme_retrieve_tool":
		return s.retrieveTool(ctx, args)
	case "reme_summarize":
		return s.summarize(ctx, args)
	default:
		return nil, fmt.Errorf("mcp_memory: unknown tool: %s", name)
	}
}

func (s *MCPMemoryToolSet) searchMemory(ctx context.Context, args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	topK := getInt(args, "top_k", 10)
	minScore := getFloat(args, "min_score", 0.1)

	opts := RetrieveOptions{
		TopK:     topK,
		MinScore: minScore,
	}

	if mt, ok := args["memory_type"].(string); ok && mt != "" {
		opts.MemoryTypes = []MemoryType{MemoryType(mt)}
	}
	if target, ok := args["memory_target"].(string); ok && target != "" {
		opts.MemoryTargets = []string{target}
	}

	nodes, err := s.Mem.RetrieveMemory(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	return memoryNodesToMCPFormat(nodes), nil
}

func (s *MCPMemoryToolSet) addMemory(ctx context.Context, args map[string]any) (any, error) {
	content, _ := args["content"].(string)
	memType, _ := args["memory_type"].(string)
	target, _ := args["memory_target"].(string)
	whenToUse, _ := args["when_to_use"].(string)
	score := getFloat(args, "score", 1.0)

	if content == "" || memType == "" {
		return nil, fmt.Errorf("mcp_memory: content and memory_type required")
	}

	node := NewMemoryNodeWithWhen(MemoryType(memType), target, content, whenToUse)
	node.Score = score
	node.TimeModified = time.Now()

	if err := s.Mem.AddMemory(ctx, node); err != nil {
		return nil, err
	}
	return map[string]any{
		"memory_id": node.MemoryID,
		"status":    "added",
	}, nil
}

func (s *MCPMemoryToolSet) retrievePersonal(ctx context.Context, args map[string]any) (any, error) {
	userName, _ := args["user_name"].(string)
	query, _ := args["query"].(string)
	topK := getInt(args, "top_k", 5)
	nodes, err := s.Mem.RetrievePersonal(ctx, userName, query, topK)
	if err != nil {
		return nil, err
	}
	return memoryNodesToMCPFormat(nodes), nil
}

func (s *MCPMemoryToolSet) retrieveProcedural(ctx context.Context, args map[string]any) (any, error) {
	taskName, _ := args["task_name"].(string)
	query, _ := args["query"].(string)
	topK := getInt(args, "top_k", 5)
	nodes, err := s.Mem.RetrieveProcedural(ctx, taskName, query, topK)
	if err != nil {
		return nil, err
	}
	return memoryNodesToMCPFormat(nodes), nil
}

func (s *MCPMemoryToolSet) retrieveTool(ctx context.Context, args map[string]any) (any, error) {
	toolName, _ := args["tool_name"].(string)
	query, _ := args["query"].(string)
	topK := getInt(args, "top_k", 5)
	nodes, err := s.Mem.RetrieveTool(ctx, toolName, query, topK)
	if err != nil {
		return nil, err
	}
	return memoryNodesToMCPFormat(nodes), nil
}

func (s *MCPMemoryToolSet) summarize(ctx context.Context, args map[string]any) (any, error) {
	userName, _ := args["user_name"].(string)
	taskName, _ := args["task_name"].(string)
	toolName, _ := args["tool_name"].(string)

	if s.ReMeMem == nil {
		return nil, fmt.Errorf("mcp_memory: SummarizeMemory requires ReMeVectorMemory with Orchestrator")
	}
	res, err := s.ReMeMem.SummarizeMemory(ctx, nil, userName, taskName, toolName)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"personal_count":   len(res.PersonalMemories),
		"procedural_count": len(res.ProceduralMemories),
		"tool_count":       len(res.ToolMemories),
		"status":           "summarized",
	}, nil
}

func memoryNodesToMCPFormat(nodes []*MemoryNode) []map[string]any {
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, map[string]any{
			"memory_id":    n.MemoryID,
			"memory_type":  string(n.MemoryType),
			"target":       n.MemoryTarget,
			"content":      n.Content,
			"when_to_use":  n.WhenToUse,
			"score":        n.Score,
			"time_created": n.TimeCreated,
		})
	}
	return out
}

func getInt(args map[string]any, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return defaultVal
}

func getFloat(args map[string]any, key string, defaultVal float64) float64 {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return defaultVal
}

