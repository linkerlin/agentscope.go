// 演示 ReMeVectorMemory：固定维度嵌入 + LocalVectorStore 检索与混合重排。
// 使用内存中的伪嵌入，无需 API Key。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/memory"
)

// fixedEmbed 为示例用确定性向量（非语义模型）。
type fixedEmbed struct{ dim int }

func (f fixedEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, f.dim)
	for i := range v {
		v[i] = 0.1 * float32((i+1)%7)
	}
	_ = text
	return v, nil
}

func (f fixedEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for range texts {
		v, err := f.Embed(ctx, "")
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func main() {
	dir, err := os.MkdirTemp("", "reme-vector-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = dir

	e := fixedEmbed{dim: 8}
	v, err := memory.NewReMeVectorMemory(cfg, memory.NewSimpleTokenCounter(), nil, e)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// 插入多条记忆（fixedEmbed 对所有文本生成相同向量，因此 BM25 将决定混合排序）
	nodes := []*memory.MemoryNode{
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "喜欢 Go 与 AgentScope"),
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "热爱 Python 数据分析"),
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "最近在学习 Go 语言并发模型"),
	}
	for _, n := range nodes {
		if err := v.AddMemory(ctx, n); err != nil {
			log.Fatal(err)
		}
	}

	query := "Go 语言"

	// 1) 纯向量检索（VectorWeight=1.0）—— 由于 fixedEmbed 相同，顺序由插入顺序决定
	resVector, err := v.RetrieveMemory(ctx, query, memory.RetrieveOptions{
		TopK:          5,
		VectorWeight:  1.0,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypePersonal},
		MemoryTargets: []string{"alice"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("=== 纯向量检索 (VectorWeight=1.0) ===")
	for _, r := range resVector {
		fmt.Printf("- score=%.4f %s\n", r.Score, r.Content)
	}

	// 2) 混合检索（VectorWeight=0.5）—— BM25 介入，"Go 语言" 相关记忆应排到最前
	resHybrid, err := v.RetrieveMemory(ctx, query, memory.RetrieveOptions{
		TopK:          5,
		VectorWeight:  0.5,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypePersonal},
		MemoryTargets: []string{"alice"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("=== 混合检索 (VectorWeight=0.5) ===")
	for _, r := range resHybrid {
		fmt.Printf("- score=%.4f %s\n", r.Score, r.Content)
	}
}
