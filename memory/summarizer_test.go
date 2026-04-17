package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// mockSummarizerModel 用于测试的模拟模型
type mockSummarizerModel struct {
	response string
	err      error
}

func (m *mockSummarizerModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.response).Build(), nil
}

func (m *mockSummarizerModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func (m *mockSummarizerModel) ModelName() string { return "mock" }

// TestNewPersonalSummarizer 测试创建个人记忆提取器
func TestNewPersonalSummarizer(t *testing.T) {
	model := &mockSummarizerModel{}
	summarizer := NewPersonalSummarizer(model, "zh")

	if summarizer == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if summarizer.Language != "zh" {
		t.Errorf("expected language 'zh', got '%s'", summarizer.Language)
	}
}

// TestNewProceduralSummarizer 测试创建程序记忆提取器
func TestNewProceduralSummarizer(t *testing.T) {
	m := &mockSummarizerModel{}
	summarizer := NewProceduralSummarizer(m, "zh")

	if summarizer == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if summarizer.Language != "zh" {
		t.Errorf("expected language 'zh', got '%s'", summarizer.Language)
	}
}

// TestNewToolSummarizer 测试创建工具记忆提取器
func TestNewToolSummarizer(t *testing.T) {
	m := &mockSummarizerModel{}
	summarizer := NewToolSummarizer(m, "zh")

	if summarizer == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if summarizer.Language != "zh" {
		t.Errorf("expected language 'zh', got '%s'", summarizer.Language)
	}
}

// TestNewMemoryDeduplicator 测试创建记忆去重器
func TestNewMemoryDeduplicator(t *testing.T) {
	embed := &mockEmbeddingModel{}
	deduplicator := NewMemoryDeduplicator(embed)

	if deduplicator == nil {
		t.Fatal("expected non-nil deduplicator")
	}
	if deduplicator.SimilarityThreshold != 0.85 {
		t.Errorf("expected threshold 0.85, got %f", deduplicator.SimilarityThreshold)
	}
}

// TestSimpleDeduplicate 测试简单去重功能
func TestSimpleDeduplicate(t *testing.T) {
	memories := []*MemoryNode{
		{MemoryID: "1", Content: "用户喜欢喝咖啡"},
		{MemoryID: "2", Content: "用户喜欢喝咖啡和茶"}, // 相似但不完全相同
		{MemoryID: "3", Content: "用户是一名工程师"},      // 不同
		{MemoryID: "4", Content: "用户喜欢喝咖啡"},      // 完全重复
	}

	unique, removed := SimpleDeduplicate(memories, 0.7)

	// 应该保留3个（1或4中的一个被去重，2虽然相似但阈值0.7下可能保留）
	if len(unique) < 2 {
		t.Errorf("expected at least 2 unique memories, got %d", len(unique))
	}
	if len(removed) < 1 {
		t.Errorf("expected at least 1 removed ID, got %d", len(removed))
	}
}

// mockEmbeddingModel 模拟嵌入模型
type mockEmbeddingModel struct {
	vectors map[string][]float32
}

func (m *mockEmbeddingModel) Embed(ctx context.Context, text string) ([]float32, error) {
	if vec, ok := m.vectors[text]; ok {
		return vec, nil
	}
	vec := make([]float32, 4)
	for i, c := range text {
		vec[i%4] += float32(c) / 1000.0
	}
	return vec, nil
}

func (m *mockEmbeddingModel) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var result [][]float32
	for _, text := range texts {
		vec, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result = append(result, vec)
	}
	return result, nil
}


func TestSummarizerAppendToMemoryMD(t *testing.T) {
	dir := t.TempDir()
	s := &Summarizer{WorkingDir: dir}
	if err := s.AppendToMemoryMD("Title", "body"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendToMemoryMD("", "body2"); err != nil {
		t.Fatal(err)
	}
}

func TestSummarizerAppendToMemoryMD_Nil(t *testing.T) {
	var s *Summarizer
	if err := s.AppendToMemoryMD("T", "b"); err == nil {
		t.Fatal("expected error for nil summarizer")
	}
	s2 := &Summarizer{}
	if err := s2.AppendToMemoryMD("T", "b"); err == nil {
		t.Fatal("expected error when WorkingDir empty")
	}
}
