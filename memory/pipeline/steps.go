package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// MemoryRetrievalStep 向量/混合检索步骤
type MemoryRetrievalStep struct {
	Store memory.VectorStore
}

func (s *MemoryRetrievalStep) Name() string { return "MemoryRetrieval" }

func (s *MemoryRetrievalStep) Execute(ctx context.Context, fc *FlowContext) error {
	if s.Store == nil || fc.Query == "" {
		return nil
	}
	nodes, err := s.Store.Search(ctx, fc.Query, memory.RetrieveOptions{
		TopK:     fc.TopK,
		MinScore: fc.MinScore,
	})
	if err != nil {
		return err
	}
	fc.RetrievedNodes = nodes
	fc.MemoryNodes = nodes
	return nil
}

// RerankMemoryStep LLM 重排序步骤
type RerankMemoryStep struct {
	Store  memory.VectorStore
	Enable bool
}

func (s *RerankMemoryStep) Name() string { return "RerankMemory" }

func (s *RerankMemoryStep) Execute(ctx context.Context, fc *FlowContext) error {
	if !s.Enable || len(fc.RetrievedNodes) <= 1 {
		fc.RerankedNodes = fc.RetrievedNodes
		return nil
	}
	sort.Slice(fc.RetrievedNodes, func(i, j int) bool {
		return fc.RetrievedNodes[i].Score > fc.RetrievedNodes[j].Score
	})
	fc.RerankedNodes = fc.RetrievedNodes
	fc.MemoryNodes = fc.RetrievedNodes
	return nil
}

// LLMRerankStep 使用 LLM 对检索结果精排（对标 ReMe Python RerankMemory）
type LLMRerankStep struct {
	Model  model.ChatModel
	TopK   int
	Enable bool
}

func (s *LLMRerankStep) Name() string { return "LLMRerankMemory" }

func (s *LLMRerankStep) Execute(ctx context.Context, fc *FlowContext) error {
	if !s.Enable || s.Model == nil || len(fc.RetrievedNodes) <= 1 {
		fc.RerankedNodes = fc.RetrievedNodes
		return nil
	}

	var sb strings.Builder
	sb.WriteString("根据查询重新排序以下记忆。\n\n")
	fmt.Fprintf(&sb, "查询: %s\n\n", fc.Query)
	sb.WriteString("记忆列表:\n")
	for i, n := range fc.RetrievedNodes {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, n.Content[:min(len(n.Content), 200)], n.Content)
	}
	sb.WriteString("\n按相关性从高到低输出序号，格式: 排序: <1> <3> <2> ...\n")

	resp, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(sb.String()).Build(),
	})
	if err != nil {
		fc.RerankedNodes = fc.RetrievedNodes
		return nil // 失败不中断流水线
	}

	indices := parseRankIndices(resp.GetTextContent())
	if len(indices) == 0 {
		fc.RerankedNodes = fc.RetrievedNodes
		return nil
	}

	reranked := make([]*memory.MemoryNode, 0, len(indices))
	seen := make(map[int]bool)
	for _, idx := range indices {
		if idx >= 0 && idx < len(fc.RetrievedNodes) && !seen[idx] {
			reranked = append(reranked, fc.RetrievedNodes[idx])
			seen[idx] = true
		}
	}
	for i, n := range fc.RetrievedNodes {
		if !seen[i] {
			reranked = append(reranked, n)
		}
	}
	if s.TopK > 0 && len(reranked) > s.TopK {
		reranked = reranked[:s.TopK]
	}

	fc.RerankedNodes = reranked
	fc.MemoryNodes = reranked
	return nil
}

func parseRankIndices(text string) []int {
	var indices []int
	for _, word := range strings.Fields(text) {
		word = strings.Trim(word, "<>,，")
		if idx := 0; true {
			if _, err := fmt.Sscanf(word, "%d", &idx); err == nil && idx > 0 {
				indices = append(indices, idx-1)
			}
		}
	}
	return indices
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MemoryValidationStep 记忆质量验证步骤
type MemoryValidationStep struct {
	Threshold float64
}

func (s *MemoryValidationStep) Name() string { return "MemoryValidation" }

func (s *MemoryValidationStep) Execute(ctx context.Context, fc *FlowContext) error {
	if len(fc.MemoryNodes) == 0 {
		return nil
	}
	var valid []*memory.MemoryNode
	for _, n := range fc.MemoryNodes {
		if n.Score >= s.Threshold {
			valid = append(valid, n)
		}
	}
	fc.ValidatedNodes = valid
	fc.MemoryNodes = valid
	return nil
}

// MemoryDeduplicationStep 记忆去重步骤
type MemoryDeduplicationStep struct {
	Dedup *memory.MemoryDeduplicator
}

func (s *MemoryDeduplicationStep) Name() string { return "MemoryDeduplication" }

func (s *MemoryDeduplicationStep) Execute(ctx context.Context, fc *FlowContext) error {
	if s.Dedup == nil || len(fc.MemoryNodes) <= 1 {
		fc.DedupedNodes = fc.MemoryNodes
		return nil
	}
	deduped, _, _ := s.Dedup.Deduplicate(ctx, fc.MemoryNodes)
	fc.DedupedNodes = deduped
	fc.MemoryNodes = deduped
	return nil
}

// MemoryAdditionStep 内存写入步骤
type MemoryAdditionStep struct {
	Store memory.VectorStore
}

func (s *MemoryAdditionStep) Name() string { return "MemoryAddition" }

func (s *MemoryAdditionStep) Execute(ctx context.Context, fc *FlowContext) error {
	if s.Store == nil || len(fc.MemoryNodes) == 0 {
		return nil
	}
	return s.Store.Insert(ctx, fc.MemoryNodes)
}

// MemoryDeletionStep 内存删除步骤
type MemoryDeletionStep struct {
	Store memory.VectorStore
}

func (s *MemoryDeletionStep) Name() string { return "MemoryDeletion" }

func (s *MemoryDeletionStep) Execute(ctx context.Context, fc *FlowContext) error {
	if s.Store == nil || len(fc.MemoryNodes) == 0 {
		return nil
	}
	for _, n := range fc.MemoryNodes {
		_ = s.Store.Delete(ctx, n.MemoryID)
	}
	return nil
}
