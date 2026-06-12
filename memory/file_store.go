package memory

import (
	"context"
	"fmt"
)

// MemorySource 标记记忆来源（对标 ReMe Python MemorySource 枚举）
type MemorySource string

const (
	SourceMemory   MemorySource = "memory"
	SourceSessions MemorySource = "sessions"
)

// FileChunk 文件分块（对标 ReMe Python MemoryChunk）
type FileChunk struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Source    MemorySource   `json:"source"`
	StartLine int            `json:"start_line"`
	EndLine   int            `json:"end_line"`
	Text      string         `json:"text"`
	Hash      string         `json:"hash"`
	Embedding []float32      `json:"embedding,omitempty"`
	Metadata  map[string]any `json:"metadata"`
}

// FileMetadata 文件元数据（对标 ReMe Python FileMetadata）
type FileMetadata struct {
	Hash    string  `json:"hash"`
	MtimeMs float64 `json:"mtime_ms"`
	Size    int64   `json:"size"`
	Path    string  `json:"path"`
	Content string  `json:"content,omitempty"`
}

// MemorySearchResult 检索结果（对标 ReMe Python MemorySearchResult）
type MemorySearchResult struct {
	Path      string         `json:"path"`
	StartLine int            `json:"start_line"`
	EndLine   int            `json:"end_line"`
	Score     float64        `json:"score"`
	Snippet   string         `json:"snippet"`
	Source    MemorySource   `json:"source"`
	RawMetric float64        `json:"raw_metric"`
	Metadata  map[string]any `json:"metadata"`
}

// MergeKey 去重键（对口 ReMe Python merge_key 属性）
func (r *MemorySearchResult) MergeKey() string {
	return fmt.Sprintf("%s:%d:%d", r.Path, r.StartLine, r.EndLine)
}

// FileStore 文件存储抽象接口（对标 ReMe Python BaseFileStore + ReMe4 BaseFileStore）
type FileStore interface {
	// CRUD
	UpsertFile(ctx context.Context, meta *FileMetadata, source MemorySource, chunks []*FileChunk) error
	DeleteFile(ctx context.Context, path string, source MemorySource) error
	DeleteFileChunks(ctx context.Context, path string, chunkIDs []string) error
	UpsertChunks(ctx context.Context, chunks []*FileChunk, source MemorySource) error

	// Query
	ListFiles(ctx context.Context, source MemorySource) ([]string, error)
	GetFileMetadata(ctx context.Context, path string, source MemorySource) (*FileMetadata, error)
	UpdateFileMetadata(ctx context.Context, meta *FileMetadata, source MemorySource) error
	GetFileChunks(ctx context.Context, path string, source MemorySource) ([]*FileChunk, error)

	// Search
	VectorSearch(ctx context.Context, query string, limit int, sources []MemorySource) ([]*MemorySearchResult, error)
	KeywordSearch(ctx context.Context, query string, limit int, sources []MemorySource) ([]*MemorySearchResult, error)
	HybridSearch(ctx context.Context, query string, limit int, sources []MemorySource, vectorWeight, candidateMultiplier float64) ([]*MemorySearchResult, error)

	// Lifecycle
	ClearAll(ctx context.Context) error
	Close() error
}

