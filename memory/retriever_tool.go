package memory

import "context"

// ToolRetriever 工具相关记忆检索薄封装
type ToolRetriever struct {
	V VectorMemory
}

// Retrieve 委托 VectorMemory.RetrieveTool
func (p *ToolRetriever) Retrieve(ctx context.Context, toolName, query string, topK int) ([]*MemoryNode, error) {
	if p == nil || p.V == nil {
		return nil, nil
	}
	return p.V.RetrieveTool(ctx, toolName, query, topK)
}
