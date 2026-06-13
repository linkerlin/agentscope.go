package memory

import "context"

// PersonalRetriever 个人记忆检索薄封装
type PersonalRetriever struct {
	V VectorMemory
}

// Retrieve 委托 VectorMemory.RetrievePersonal
func (p *PersonalRetriever) Retrieve(ctx context.Context, userName, query string, topK int) ([]*MemoryNode, error) {
	if p == nil || p.V == nil {
		return nil, nil
	}
	return p.V.RetrievePersonal(ctx, userName, query, topK)
}
