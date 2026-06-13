package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/linkerlin/agentscope.go/memory/vector"
	"github.com/linkerlin/agentscope.go/message"
)

// NewMemoryNode 创建记忆节点并生成 ID
func NewMemoryNode(memType MemoryType, target, content string) *MemoryNode {
	now := time.Now()
	return &MemoryNode{
		MemoryID:     GenerateMemoryID(content + "|" + target),
		MemoryType:   memType,
		MemoryTarget: target,
		Content:      content,
		TimeCreated:  now,
		TimeModified: now,
		Metadata:     make(map[string]any),
	}
}

// NewMemoryNodeWithWhen 创建带 whenToUse 的记忆节点；whenToUse 非空时将用于向量嵌入
func NewMemoryNodeWithWhen(memType MemoryType, target, content, whenToUse string) *MemoryNode {
	n := NewMemoryNode(memType, target, content)
	n.WhenToUse = whenToUse
	return n
}

// GenerateMemoryID 由内容生成短 ID（16 hex）
func GenerateMemoryID(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// MemoryType, MemoryNode, RetrieveOptions, VectorStore re-exported from vector subpackage
// (see memory/vector/types.go). Completes light split pilot.
type (
	MemoryType      = vector.MemoryType
	MemoryNode      = vector.MemoryNode
	RetrieveOptions = vector.RetrieveOptions
	VectorStore     = vector.VectorStore
)

const (
	MemoryTypePersonal   MemoryType = "personal"
	MemoryTypeProcedural MemoryType = "procedural"
	MemoryTypeTool       MemoryType = "tool"
	MemoryTypeSummary    MemoryType = "summary"
	MemoryTypeHistory    MemoryType = "history"
	MemoryTypeIdentity   MemoryType = "identity"
	// Evolution asset types (aligned with evolver narrativeMemory / gene/capsule/event memory)
	MemoryTypeGene     MemoryType = "gene"
	MemoryTypeCapsule  MemoryType = "capsule"
	MemoryTypeEvoEvent MemoryType = "evolution_event"
)

// (MemoryNode now alias to vector.MemoryNode from types.go)

// EmbeddingContent 返回应被向量嵌入的文本。
// 规则（对标 ReMe Python to_vector_node）：
//   - 若 WhenToUse 非空，使用 WhenToUse（检索时以触发条件匹配）
//   - 否则使用 Content（直接以内容匹配）
// EmbeddingContent defined in vector/types.go (alias prevents method definition here)

// CompactSummary 结构化压缩摘要
type CompactSummary struct {
	Goal            string   `json:"goal"`
	Constraints     []string `json:"constraints"`
	Progress        string   `json:"progress"`
	KeyDecisions    []string `json:"key_decisions"`
	NextSteps       []string `json:"next_steps"`
	CriticalContext []string `json:"critical_context"`
	Raw             string   `json:"raw"`
}

// ContextCheckResult 上下文检查结果
type ContextCheckResult struct {
	MessagesToCompact []*message.Msg
	MessagesToKeep    []*message.Msg
	IsValid           bool
	TotalTokens       int
	Threshold         int
	// 完整性检查报告
	Completeness *ContextCompletenessReport
}

// TokenStats Token 统计
type TokenStats struct {
	TotalMessages           int     `json:"total_messages"`
	CompressedSummaryTokens int     `json:"compressed_summary_tokens"`
	MessagesTokens          int     `json:"messages_tokens"`
	EstimatedTokens         int     `json:"estimated_tokens"`
	MaxInputLength          int     `json:"max_input_length"`
	ContextUsageRatio       float64 `json:"context_usage_ratio"`
}

// (RetrieveOptions now alias to vector.RetrieveOptions)

// CompactOptions 压缩选项
type CompactOptions struct {
	MaxInputLength   int     `json:"max_input_length"`
	CompactRatio     float64 `json:"compact_ratio"`
	ReserveTokens    int     `json:"reserve_tokens"`
	PreviousSummary  string  `json:"previous_summary"`
	Language         string  `json:"language"`
	AddThinkingBlock bool    `json:"add_thinking_block"`
}

// SummarizeResult 编排器 summarize 结果
type SummarizeResult struct {
	PersonalMemories   []*MemoryNode
	ProceduralMemories []*MemoryNode
	ToolMemories       []*MemoryNode
	UpdatedProfiles    map[string]map[string]any // user -> profile
	AddedHistory       *MemoryNode
}

// OrchestratorConfig 编排器配置
type OrchestratorConfig struct {
	EnablePersonal       bool
	EnableProcedural     bool
	EnableTool           bool
	EnableProfile        bool
	EnableHistory        bool
	RetrieveTopK         int
	MinScore             float64
	DeduplicateThreshold float64
}

// DefaultOrchestratorConfig 返回默认配置
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		EnablePersonal:       true,
		EnableProcedural:     true,
		EnableTool:           true,
		EnableProfile:        true,
		EnableHistory:        true,
		RetrieveTopK:         20,
		MinScore:             0.1,
		DeduplicateThreshold: 0.85,
	}
}

// MessageMark 消息标记
type MessageMark string

const (
	MarkCompressed MessageMark = "compressed"
	MarkImportant  MessageMark = "important"
	MarkDeleted    MessageMark = "deleted"
)
