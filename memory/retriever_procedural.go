package memory

import "context"

// ProceduralRetriever 任务/程序记忆检索薄封装
type ProceduralRetriever struct {
	V VectorMemory
}

// Retrieve 委托 VectorMemory.RetrieveProcedural
func (p *ProceduralRetriever) Retrieve(ctx context.Context, taskName, query string, topK int) ([]*MemoryNode, error) {
	if p == nil || p.V == nil {
		return nil, nil
	}
	return p.V.RetrieveProcedural(ctx, taskName, query, topK)
}
