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
	store VectorStore
	orch  Orchestrator
}

// NewReMeVectorMemory 创建向量记忆；store 为空则使用 LocalVectorStore(embed)
func NewReMeVectorMemory(cfg ReMeFileConfig, counter TokenCounter, store VectorStore, embed EmbeddingModel) (*ReMeVectorMemory, error) {
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

// AddMemory 写入向量库并同步维护 FTS5
func (v *ReMeVectorMemory) AddMemory(ctx context.Context, node *MemoryNode) error {
	if v == nil || v.store == nil {
		return ErrEmbeddingRequired
	}
	if err := v.store.Insert(ctx, []*MemoryNode{node}); err != nil {
		return err
	}
	if v.ReMeFileMemory != nil && v.ReMeFileMemory.fts != nil {
		_ = v.ReMeFileMemory.fts.Insert(node)
	}
	return nil
}

// RetrieveMemory 语义检索；若 VectorWeight 在 (0,1) 则做混合重排
func (v *ReMeVectorMemory) RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if v == nil || v.store == nil {
		return nil, ErrEmbeddingRequired
	}
	// 两阶段混合检索：向量召回候选集扩大 3 倍
	vectorOpts := opts
	if vectorOpts.TopK <= 0 {
		vectorOpts.TopK = 10
	}
	vectorOpts.TopK *= 3
	nodes, err := v.store.Search(ctx, query, vectorOpts)
	if err != nil {
		return nil, err
	}
	w := opts.VectorWeight
	if w > 0 && w < 1 && len(nodes) > 0 && v.fts != nil {
		nodes = RankMemoryNodesHybrid(nodes, query, w, v.fts)
	}
	if opts.TopK > 0 && len(nodes) > opts.TopK {
		nodes = nodes[:opts.TopK]
	}
	return nodes, nil
}

// UpdateMemory 更新节点并同步维护 FTS5
func (v *ReMeVectorMemory) UpdateMemory(ctx context.Context, node *MemoryNode) error {
	if v == nil || v.store == nil {
		return ErrEmbeddingRequired
	}
	if err := v.store.Update(ctx, node); err != nil {
		return err
	}
	if v.ReMeFileMemory != nil && v.ReMeFileMemory.fts != nil {
		_ = v.ReMeFileMemory.fts.Update(node)
	}
	return nil
}

// DeleteMemory 删除节点并同步维护 FTS5
func (v *ReMeVectorMemory) DeleteMemory(ctx context.Context, memoryID string) error {
	if v == nil || v.store == nil {
		return ErrEmbeddingRequired
	}
	if err := v.store.Delete(ctx, memoryID); err != nil {
		return err
	}
	if v.ReMeFileMemory != nil && v.ReMeFileMemory.fts != nil {
		_ = v.ReMeFileMemory.fts.Delete(memoryID)
	}
	return nil
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
	if lv, ok := v.store.(*LocalVectorStore); ok {
		return lv.WriteSnapshot(path)
	}
	return nil
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
	if lv, ok := v.store.(*LocalVectorStore); ok {
		return lv.ReadSnapshot(path)
	}
	return nil
}

// SetOrchestrator 注入编排器
func (v *ReMeVectorMemory) SetOrchestrator(o Orchestrator) {
	v.orch = o
}

// VectorStore 返回底层向量存储（供 Handler 等外部组件使用）
func (v *ReMeVectorMemory) VectorStore() VectorStore {
	return v.store
}

// SummarizeMemory 端到端记忆提取与持久化
func (v *ReMeVectorMemory) SummarizeMemory(ctx context.Context, msgs []*message.Msg, userName, taskName, toolName string) (*SummarizeResult, error) {
	if v.orch == nil {
		return nil, errors.New("memory: orchestrator not set")
	}
	return v.orch.Summarize(ctx, msgs, userName, taskName, toolName)
}

// RetrieveMemoryUnified 统一检索入口
func (v *ReMeVectorMemory) RetrieveMemoryUnified(ctx context.Context, query string, userName, taskName, toolName string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if v.orch == nil {
		return nil, errors.New("memory: orchestrator not set")
	}
	return v.orch.Retrieve(ctx, query, userName, taskName, toolName, opts)
}

// NewReMeVectorMemoryWithOrchestrator 创建带编排器的向量记忆
func NewReMeVectorMemoryWithOrchestrator(cfg ReMeFileConfig, counter TokenCounter, store VectorStore, embed EmbeddingModel, orch Orchestrator) (*ReMeVectorMemory, error) {
	v, err := NewReMeVectorMemory(cfg, counter, store, embed)
	if err != nil {
		return nil, err
	}
	v.orch = orch
	return v, nil
}

// Close 关闭底层资源（FTS 数据库连接等）
func (v *ReMeVectorMemory) Close() error {
	if v == nil || v.ReMeFileMemory == nil {
		return nil
	}
	return v.ReMeFileMemory.Close()
}

var _ VectorMemory = (*ReMeVectorMemory)(nil)
