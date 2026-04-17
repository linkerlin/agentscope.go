package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// KnowledgeRetrievalTool 允许 Agent 自主调用知识检索
type KnowledgeRetrievalTool struct {
	Retriever   MemoryRAG
	DefaultTopK int
}

// NewKnowledgeRetrievalTool 创建知识检索工具
func NewKnowledgeRetrievalTool(retriever MemoryRAG, defaultTopK int) *KnowledgeRetrievalTool {
	if defaultTopK <= 0 {
		defaultTopK = 3
	}
	return &KnowledgeRetrievalTool{Retriever: retriever, DefaultTopK: defaultTopK}
}

// Name 返回工具名
func (t *KnowledgeRetrievalTool) Name() string { return "retrieve_knowledge" }

// Description 返回工具描述
func (t *KnowledgeRetrievalTool) Description() string {
	return "Retrieve relevant knowledge documents based on a query."
}

// Spec 返回工具 JSON Schema
func (t *KnowledgeRetrievalTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
				"top_k": map[string]any{
					"type":        "number",
					"description": "Number of top results to return",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute 执行检索并返回结果
func (t *KnowledgeRetrievalTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	if t.Retriever == nil {
		return nil, fmt.Errorf("rag: retriever not configured")
	}
	query, _ := input["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("rag: query is required")
	}
	topK := t.DefaultTopK
	if tk, ok := input["top_k"].(float64); ok && tk > 0 {
		topK = int(tk)
	} else if tk, ok := input["top_k"].(int); ok && tk > 0 {
		topK = tk
	}

	msgs, err := t.Retriever.Retrieve(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	var parts []string
	for i, m := range msgs {
		text := m.GetTextContent()
		if text != "" {
			parts = append(parts, fmt.Sprintf("[%d] %s", i+1, text))
		}
	}
	if len(parts) == 0 {
		return tool.NewTextResponse("No relevant knowledge found."), nil
	}
	return tool.NewTextResponse(strings.Join(parts, "\n\n")), nil
}
