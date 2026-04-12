package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/message"
)

// VectorMemory 在 ReMeMemory 之上增加向量 CRUD 与类型化检索
type VectorMemory interface {
	ReMeMemory

	AddMemory(ctx context.Context, node *MemoryNode) error
	RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
	UpdateMemory(ctx context.Context, node *MemoryNode) error
	DeleteMemory(ctx context.Context, memoryID string) error

	RetrievePersonal(ctx context.Context, userName, query string, topK int) ([]*MemoryNode, error)
	RetrieveProcedural(ctx context.Context, taskName, query string, topK int) ([]*MemoryNode, error)
	RetrieveTool(ctx context.Context, toolName, query string, topK int) ([]*MemoryNode, error)
}

// ReMeVectorMemory 组合文件记忆与向量存储
type ReMeVectorMemory struct {
	*ReMeFileMemory
	store *LocalVectorStore
}

// NewReMeVectorMemory 创建向量记忆；store 为空则使用 LocalVectorStore(embed)
func NewReMeVectorMemory(cfg ReMeFileConfig, counter TokenCounter, store *LocalVectorStore, embed EmbeddingModel) (*ReMeVectorMemory, error) {
	f, err := NewReMeFileMemory(cfg, counter)
	if err != nil {
		return nil, err
	}
	if store == nil {
		if embed == nil {
			return nil, ErrEmbeddingRequired
		}
		store = NewLocalVectorStore(embed)
	}
	return &ReMeVectorMemory{ReMeFileMemory: f, store: store}, nil
}

// AddMemory 写入向量库
func (v *ReMeVectorMemory) AddMemory(ctx context.Context, node *MemoryNode) error {
	if v == nil || v.store == nil {
		return ErrEmbeddingRequired
	}
	return v.store.Insert(ctx, []*MemoryNode{node})
}

// RetrieveMemory 语义检索；若 VectorWeight 在 (0,1) 则做混合重排
func (v *ReMeVectorMemory) RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if v == nil || v.store == nil {
		return nil, ErrEmbeddingRequired
	}
	nodes, err := v.store.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	w := opts.VectorWeight
	if w > 0 && w < 1 && len(nodes) > 0 {
		nodes = RankMemoryNodesHybrid(nodes, query, w)
	}
	return nodes, nil
}

// UpdateMemory 更新节点
func (v *ReMeVectorMemory) UpdateMemory(ctx context.Context, node *MemoryNode) error {
	if v == nil || v.store == nil {
		return ErrEmbeddingRequired
	}
	return v.store.Update(ctx, node)
}

// DeleteMemory 删除节点
func (v *ReMeVectorMemory) DeleteMemory(ctx context.Context, memoryID string) error {
	if v == nil || v.store == nil {
		return ErrEmbeddingRequired
	}
	return v.store.Delete(ctx, memoryID)
}

// RetrievePersonal 个人偏好类记忆
func (v *ReMeVectorMemory) RetrievePersonal(ctx context.Context, userName, query string, topK int) ([]*MemoryNode, error) {
	return v.RetrieveMemory(ctx, query, RetrieveOptions{
		TopK:          topK,
		MemoryTypes:   []MemoryType{MemoryTypePersonal},
		MemoryTargets: []string{userName},
	})
}

// RetrieveProcedural 任务/程序类记忆
func (v *ReMeVectorMemory) RetrieveProcedural(ctx context.Context, taskName, query string, topK int) ([]*MemoryNode, error) {
	return v.RetrieveMemory(ctx, query, RetrieveOptions{
		TopK:          topK,
		MemoryTypes:   []MemoryType{MemoryTypeProcedural},
		MemoryTargets: []string{taskName},
	})
}

// RetrieveTool 工具相关记忆
func (v *ReMeVectorMemory) RetrieveTool(ctx context.Context, toolName, query string, topK int) ([]*MemoryNode, error) {
	return v.RetrieveMemory(ctx, query, RetrieveOptions{
		TopK:          topK,
		MemoryTypes:   []MemoryType{MemoryTypeTool},
		MemoryTargets: []string{toolName},
	})
}

// MessagesToMemoryNodes 将对话批量转为记忆节点（辅助）
func MessagesToMemoryNodes(msgs []*message.Msg, memType MemoryType, target string) []*MemoryNode {
	var out []*MemoryNode
	for _, m := range msgs {
		if m == nil {
			continue
		}
		txt := m.GetTextContent()
		if txt == "" {
			continue
		}
		n := NewMemoryNode(memType, target, txt)
		n.MessageTime = m.CreatedAt
		out = append(out, n)
	}
	return out
}

// SaveTo 持久化文件记忆快照，并将会话对应的向量索引写入 sessions/<id>.vector.json
func (v *ReMeVectorMemory) SaveTo(sessionID string) error {
	if v == nil || v.ReMeFileMemory == nil {
		return errors.New("memory: nil ReMeVectorMemory")
	}
	if err := v.ReMeFileMemory.SaveTo(sessionID); err != nil {
		return err
	}
	if v.store == nil {
		return nil
	}
	path := filepath.Join(v.sessionsPath, sessionID+".vector.json")
	return v.store.WriteSnapshot(path)
}

// LoadFrom 加载会话快照；若存在向量快照则恢复 LocalVectorStore
func (v *ReMeVectorMemory) LoadFrom(sessionID string) error {
	if v == nil || v.ReMeFileMemory == nil {
		return errors.New("memory: nil ReMeVectorMemory")
	}
	if err := v.ReMeFileMemory.LoadFrom(sessionID); err != nil {
		return err
	}
	if v.store == nil {
		return nil
	}
	path := filepath.Join(v.sessionsPath, sessionID+".vector.json")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return v.store.ReadSnapshot(path)
}

var _ VectorMemory = (*ReMeVectorMemory)(nil)
