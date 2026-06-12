package vector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	MemoryTypePersonal   MemoryType = "personal"
	MemoryTypeProcedural MemoryType = "procedural"
	MemoryTypeTool       MemoryType = "tool"
	MemoryTypeProfile    MemoryType = "profile"
	MemoryTypeHistory    MemoryType = "history"
	MemoryTypeCapsule    MemoryType = "capsule"
	MemoryTypeEvoEvent   MemoryType = "evolution_event"
)

// MemoryNode 向量记忆节点
type MemoryNode struct {
	MemoryID     string         `json:"memory_id"`
	MemoryType   MemoryType     `json:"memory_type"`
	MemoryTarget string         `json:"memory_target"`
	WhenToUse    string         `json:"when_to_use"`
	Content      string         `json:"content"`
	MessageTime  time.Time      `json:"message_time"`
	RefMemoryID  string         `json:"ref_memory_id"`
	TimeCreated  time.Time      `json:"time_created"`
	TimeModified time.Time      `json:"time_modified"`
	Author       string         `json:"author"`
	Score        float64        `json:"score"`
	Vector       []float32      `json:"vector,omitempty"`
	Metadata     map[string]any `json:"metadata"`
}

// EmbeddingContent 返回应被向量嵌入的文本。
// 规则（对标 ReMe Python to_vector_node）：
func (n *MemoryNode) EmbeddingContent() string {
	if n == nil {
		return ""
	}
	if n.WhenToUse != "" {
		return n.WhenToUse
	}
	return n.Content
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

// VectorStore 向量记忆存储接口 (moved to vector subpackage)
type VectorStore interface {
	Insert(ctx context.Context, nodes []*MemoryNode) error
	Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
	Get(ctx context.Context, memoryID string) (*MemoryNode, error)
	Update(ctx context.Context, node *MemoryNode) error
	Delete(ctx context.Context, memoryID string) error
	DeleteAll(ctx context.Context) error
}

// EmbeddingModel 文本嵌入（向量记忆依赖） - duplicated in vector for self-contained subpackage (no import cycle)
type EmbeddingModel interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// GenerateMemoryID 由内容生成短 ID（16 hex） - moved to vector for split
func GenerateMemoryID(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// CosineSimilarity 计算两 float32 向量的余弦相似度；长度不一致或空向量时返回 0。 - moved to vector
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

