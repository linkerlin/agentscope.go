package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

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

// GenerateMemoryID 由内容生成短 ID（16 hex）
func GenerateMemoryID(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// MemoryType 向量记忆类型
type MemoryType string

const (
	MemoryTypePersonal   MemoryType = "personal"
	MemoryTypeProcedural MemoryType = "procedural"
	MemoryTypeTool       MemoryType = "tool"
	MemoryTypeSummary    MemoryType = "summary"
)

// MemoryNode 向量记忆节点
type MemoryNode struct {
	MemoryID      string         `json:"memory_id"`
	MemoryType    MemoryType     `json:"memory_type"`
	MemoryTarget  string         `json:"memory_target"`
	WhenToUse     string         `json:"when_to_use"`
	Content       string         `json:"content"`
	MessageTime   time.Time      `json:"message_time"`
	RefMemoryID   string         `json:"ref_memory_id"`
	TimeCreated   time.Time      `json:"time_created"`
	TimeModified  time.Time      `json:"time_modified"`
	Author        string         `json:"author"`
	Score         float64        `json:"score"`
	Vector        []float32      `json:"vector,omitempty"`
	Metadata      map[string]any `json:"metadata"`
}

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

// RetrieveOptions 检索选项
type RetrieveOptions struct {
	TopK             int          `json:"top_k"`
	MinScore         float64      `json:"min_score"`
	MemoryTypes      []MemoryType `json:"memory_types,omitempty"`
	MemoryTargets    []string     `json:"memory_targets,omitempty"`
	EnableTimeFilter bool         `json:"enable_time_filter"`
	VectorWeight     float64      `json:"vector_weight"`
}

// CompactOptions 压缩选项
type CompactOptions struct {
	MaxInputLength   int     `json:"max_input_length"`
	CompactRatio     float64 `json:"compact_ratio"`
	ReserveTokens    int     `json:"reserve_tokens"`
	PreviousSummary  string  `json:"previous_summary"`
	Language         string  `json:"language"`
	AddThinkingBlock bool    `json:"add_thinking_block"`
}

// MessageMark 消息标记
type MessageMark string

const (
	MarkCompressed MessageMark = "compressed"
	MarkImportant  MessageMark = "important"
	MarkDeleted    MessageMark = "deleted"
)
