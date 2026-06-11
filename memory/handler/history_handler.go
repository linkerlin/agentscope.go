package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// HistoryHandler 管理会话历史记录的读写（对标 ReMe Python AddHistory / ReadHistory / ReadHistoryV2）
type HistoryHandler struct {
	Store memory.VectorStore
}

// NewHistoryHandler 创建历史记录处理器
func NewHistoryHandler(store memory.VectorStore) *HistoryHandler {
	return &HistoryHandler{Store: store}
}

// AddHistory 将消息列表封装为 HISTORY 类型节点写入向量库。
// 原始消息以 JSON 序列化后存入 Content，元信息存入 Metadata。
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
	node := memory.NewMemoryNode(memory.MemoryTypeHistory, memoryTarget, string(content))
	node.Author = author
	node.WhenToUse = "conversation history for " + memoryTarget
	node.Metadata["history_type"] = "conversation"
	node.Metadata["message_count"] = len(payload)
	node.TimeModified = time.Now()
	if h.Store != nil {
		if err := h.Store.Insert(ctx, []*memory.MemoryNode{node}); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// AddIdentity 添加或更新身份类记忆（对标 ReMe IDENTITY 类型）
func (h *HistoryHandler) AddIdentity(ctx context.Context, target string, key string, value string) (*memory.MemoryNode, error) {
	node := memory.NewMemoryNodeWithWhen(memory.MemoryTypeIdentity, target,
		key+": "+value,
		"identity attribute for "+target)
	node.Metadata["identity_key"] = key
	node.Metadata["identity_value"] = value
	node.TimeModified = time.Now()
	if h.Store != nil {
		if err := h.Store.Insert(ctx, []*memory.MemoryNode{node}); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// ReadHistory 按目标读取历史记录节点（HISTORY 类型）
func (h *HistoryHandler) ReadHistory(ctx context.Context, memoryTarget string, limit int) ([]*memory.MemoryNode, error) {
	if h.Store == nil {
		return nil, nil
	}
	return h.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          limit,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypeHistory},
		MemoryTargets: []string{memoryTarget},
	})
}

// ReadHistoryByID 根据 history_id 读取原始对话内容
func (h *HistoryHandler) ReadHistoryByID(ctx context.Context, historyID string) (*memory.MemoryNode, error) {
	if h.Store == nil {
		return nil, nil
	}
	node, err := h.Store.Get(ctx, historyID)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ParseHistoryMessages 将 HISTORY 节点中的 JSON 消息还原为 message.Msg 列表
func (h *HistoryHandler) ParseHistoryMessages(node *memory.MemoryNode) ([]*message.Msg, error) {
	if node == nil {
		return nil, nil
	}
	var rawMessages []json.RawMessage
	if err := json.Unmarshal([]byte(node.Content), &rawMessages); err != nil {
		return nil, err
	}
	msgs := make([]*message.Msg, 0, len(rawMessages))
	for _, raw := range rawMessages {
		var m message.Msg
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

// ReadIdentity 读取用户的身份记忆（IDENTITY 类型）
func (h *HistoryHandler) ReadIdentity(ctx context.Context, target string) (map[string]string, error) {
	if h.Store == nil {
		return nil, nil
	}
	nodes, err := h.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          50,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypeIdentity},
		MemoryTargets: []string{target},
	})
	if err != nil {
		return nil, err
	}
	identity := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if key, ok := n.Metadata["identity_key"].(string); ok {
			if val, ok := n.Metadata["identity_value"].(string); ok {
				identity[key] = val
			}
		}
	}
	return identity, nil
}

// ReadHistoryV2 分页读取历史记录（对标 ReMe Python ReadHistoryV2）
func (h *HistoryHandler) ReadHistoryV2(ctx context.Context, memoryTarget string, page, pageSize int) ([]*memory.MemoryNode, int, error) {
	if h.Store == nil {
		return nil, 0, nil
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	all, err := h.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          200,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypeHistory},
		MemoryTargets: []string{memoryTarget},
	})
	if err != nil {
		return nil, 0, err
	}

	total := len(all)
	start := page * pageSize
	if start >= total {
		return nil, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}
