package handler

import (
	"context"
	"encoding/json"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// HistoryHandler 管理会话历史记录的读写
type HistoryHandler struct {
	Store memory.VectorStore
}

// NewHistoryHandler 创建历史记录处理器
func NewHistoryHandler(store memory.VectorStore) *HistoryHandler {
	return &HistoryHandler{Store: store}
}

// AddHistory 将消息列表封装为历史节点写入向量库
func (h *HistoryHandler) AddHistory(ctx context.Context, msgs []*message.Msg, memoryTarget, author string) (*memory.MemoryNode, error) {
	var payload []json.RawMessage
	for _, m := range msgs {
		if m == nil {
			continue
		}
		data, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		payload = append(payload, data)
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	node := memory.NewMemoryNode(memory.MemoryTypeSummary, memoryTarget, string(content))
	node.Author = author
	node.Metadata["history_type"] = "conversation"
	if h.Store != nil {
		if err := h.Store.Insert(ctx, []*memory.MemoryNode{node}); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// ReadHistory 读取指定目标的历史记录节点
func (h *HistoryHandler) ReadHistory(ctx context.Context, memoryTarget string, limit int) ([]*memory.MemoryNode, error) {
	if h.Store == nil {
		return nil, nil
	}
	return h.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          limit,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypeSummary},
		MemoryTargets: []string{memoryTarget},
	})
}
