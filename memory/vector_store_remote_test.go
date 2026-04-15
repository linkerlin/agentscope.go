package memory

import (
	"testing"
)

// 以下测试仅验证构造函数在缺少嵌入模型时返回错误，
// 因为远程服务可能未运行，不做真实网络调用。

func TestQdrantVectorStoreNilEmbed(t *testing.T) {
	_, err := NewQdrantVectorStore("localhost", 6334, "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

func TestChromaVectorStoreNilEmbed(t *testing.T) {
	_, err := NewChromaVectorStore("http://localhost:8000", "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

func TestESVectorStoreNilEmbed(t *testing.T) {
	_, err := NewESVectorStore("http://localhost:9200", "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

func TestPGVectorStoreNilEmbed(t *testing.T) {
	_, err := NewPGVectorStore("postgres://localhost/test", "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}
