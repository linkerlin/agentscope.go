// 演示仅文件型 ReMe 记忆：工作目录、Add/GetMemoryForPrompt、会话 SaveTo/LoadFrom。
// 不调用真实 LLM，适合离线查看目录与快照行为。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

func main() {
	dir, err := os.MkdirTemp("", "reme-file-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = dir

	m, err := memory.NewReMeFileMemory(cfg, memory.NewSimpleTokenCounter())
	if err != nil {
		log.Fatal(err)
	}
	m.SetLongTermMemory("用户偏好：简洁回答。")

	u := message.NewMsg().Role(message.RoleUser).TextContent("你好，请介绍 ReMe 文件记忆。").Build()
	if err := m.Add(u); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	hist, err := m.GetAll()
	if err != nil {
		log.Fatal(err)
	}
	msgs, _, err := m.PreReasoningPrepare(ctx, hist)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PreReasoningPrepare 返回 %d 条消息\n", len(msgs))

	promptView, err := m.GetMemoryForPrompt(true)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("GetMemoryForPrompt(true) 首条角色: %v\n", promptView[0].Role)

	if err := m.SaveTo("demo-session"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("已写入会话快照:", dir)

	m2, _ := memory.NewReMeFileMemory(cfg, memory.NewSimpleTokenCounter())
	if err := m2.LoadFrom("demo-session"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("已从快照恢复长期记忆字段（与 SaveTo 一致）")
}
