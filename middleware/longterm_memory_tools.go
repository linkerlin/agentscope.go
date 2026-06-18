package middleware

import (
	"context"
	"fmt"
	"strconv"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// memoryToolConfig is the minimal config the memory tools read from the
// middleware (kept here to avoid exporting accessor methods).
type memoryToolConfig interface {
	memoryBackend() LongTermMemory
	memoryUserID() string
	memoryAgentID() string
	memoryTopK() int
}

func (m *LongTermMemoryMiddleware) memoryBackend() LongTermMemory { return m.Backend }
func (m *LongTermMemoryMiddleware) memoryUserID() string          { return m.UserID }
func (m *LongTermMemoryMiddleware) memoryAgentID() string         { return m.AgentID }
func (m *LongTermMemoryMiddleware) memoryTopK() int               { return m.TopK }

// NewMemorySearchTool builds the "search_memory" tool for agent_control / both
// modes. It retrieves memories relevant to the given query.
func NewMemorySearchTool(backend LongTermMemory, cfg memoryToolConfig) tool.Tool {
	userID := cfg.memoryUserID()
	agentID := cfg.memoryAgentID()
	defaultTopK := cfg.memoryTopK()
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The query text to search long-term memory for.",
			},
			"top_k": map[string]any{
				"type":        "integer",
				"description": "Max number of memories to return (optional).",
			},
		},
		"required": []string{"query"},
	}
	return tool.NewFunctionTool(
		"search_memory",
		"Search the agent's long-term memory for facts about the user that may be relevant.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			query, _ := input["query"].(string)
			if query == "" {
				return textResponse("search_memory: empty query"), nil
			}
			topK := defaultTopK
			if v, ok := numericInt(input["top_k"]); ok && v > 0 {
				topK = v
			}
			memories, err := backend.Search(ctx, query, SearchOptions{
				TopK:    topK,
				UserID:  userID,
				AgentID: agentID,
			})
			if err != nil {
				return textResponse(fmt.Sprintf("search_memory error: %v", err)), nil
			}
			return textResponse(formatMemories(memories)), nil
		},
	)
}

// NewMemoryAddTool builds the "add_memory" tool for agent_control / both modes.
// It persists a durable fact about the user.
func NewMemoryAddTool(backend LongTermMemory, cfg memoryToolConfig) tool.Tool {
	userID := cfg.memoryUserID()
	agentID := cfg.memoryAgentID()
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "The durable fact about the user to remember.",
			},
		},
		"required": []string{"text"},
	}
	return tool.NewFunctionTool(
		"add_memory",
		"Persist a durable fact about the user into long-term memory.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			text, _ := input["text"].(string)
			if text == "" {
				return textResponse("add_memory: empty text"), nil
			}
			if err := backend.Add(ctx, []string{text}, AddOptions{
				UserID:  userID,
				AgentID: agentID,
			}); err != nil {
				return textResponse(fmt.Sprintf("add_memory error: %v", err)), nil
			}
			return textResponse("memory added"), nil
		},
	)
}

func textResponse(text string) *tool.Response {
	return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock(text)}}
}

func formatMemories(memories []Memory) string {
	if len(memories) == 0 {
		return "no memories found"
	}
	out := fmt.Sprintf("%d memory/memories found:\n", len(memories))
	for i, mem := range memories {
		out += fmt.Sprintf("%d. %s\n", i+1, mem.Text)
	}
	return out
}

// numericInt coerces a map[string]any value (which may arrive as float64 from
// JSON) to an int.
func numericInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}
