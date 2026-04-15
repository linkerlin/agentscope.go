// 演示 MemoryOrchestrator 端到端工作流：
// SummarizeMemory（提取+去重+写入+更新 Profile）与 RetrieveMemoryUnified（统一检索）。
// 使用伪嵌入与 mock LLM，无需 API Key。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/memory/handler"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// mockModel 模拟 ChatModel，返回固定观察文本
type mockModel struct{}

func (m *mockModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().
		Role(message.RoleAssistant).
		TextContent("信息：<1> <> <用户喜欢Go语言> <技术偏好>\n信息：<2> <> <用户是后端工程师> <职业>").
		Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func (m *mockModel) ModelName() string { return "mock" }

// fixedEmbed 确定性伪嵌入（非语义）
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
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "reme-orch-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = dir

	// 1) 创建 ReMeVectorMemory
	embed := fixedEmbed{dim: 8}
	v, err := memory.NewReMeVectorMemory(cfg, memory.NewSimpleTokenCounter(), nil, embed)
	if err != nil {
		log.Fatal(err)
	}

	// 2) 创建 Handler 与 Orchestrator
	memTool := handler.NewMemoryHandler(v.VectorStore())
	profileTool := handler.NewProfileHandler(dir + "/profile")
	historyTool := handler.NewHistoryHandler(v.VectorStore())

	m := &mockModel{}
	ps := memory.NewPersonalSummarizer(m, "zh")
	proc := memory.NewProceduralSummarizer(m, "zh")
	ts := memory.NewToolSummarizer(m, "zh")

	o := handler.NewMemoryOrchestrator(ps, proc, ts, memTool, profileTool, historyTool, nil)
	v.SetOrchestrator(o)

	// 3) SummarizeMemory
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("你好，我喜欢用 Go 写后端服务。").Build(),
	}

	res, err := v.SummarizeMemory(ctx, msgs, "alice", "", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("SummarizeMemory 提取到 %d 条个人记忆\n", len(res.PersonalMemories))
	for _, n := range res.PersonalMemories {
		fmt.Printf("  - [%s] %s\n", n.Metadata["keywords"], n.Content)
	}

	// 4) RetrieveMemoryUnified
	ret, err := v.RetrieveMemoryUnified(ctx, "Go", "alice", "", "", memory.RetrieveOptions{TopK: 5})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("RetrieveMemoryUnified 命中 %d 条\n", len(ret))
	for _, n := range ret {
		fmt.Printf("  - score=%.4f %s\n", n.Score, n.Content)
	}

	// 5) 验证 Profile
	if res.UpdatedProfiles["alice"] != nil {
		fmt.Printf("Profile 已更新: %v\n", res.UpdatedProfiles["alice"])
	}
}
