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

	n1 := memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "喜欢 Go 与 AgentScope")
	if err := v.AddMemory(ctx, n1); err != nil {
		log.Fatal(err)
	}

	res, err := v.RetrieveMemory(ctx, "Go", memory.RetrieveOptions{
		TopK:          5,
		VectorWeight:  0.6,
		MemoryTypes:   []memory.MemoryType{memory.MemoryTypePersonal},
		MemoryTargets: []string{"alice"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("RetrieveMemory 命中 %d 条\n", len(res))
	for _, r := range res {
		fmt.Printf("- score=%.4f %s\n", r.Score, r.Content)
	}
}
