// 演示 PersonalSummarizer、ProceduralSummarizer、ToolSummarizer 的使用
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// 简单的模拟模型实现
type mockModel struct {
	name string
}

func (m *mockModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	// 模拟不同的响应
	lastMsg := msgs[len(msgs)-1].GetTextContent()

	if contains(lastMsg, "个人") || contains(lastMsg, "observation") {
		return message.NewMsg().Role(message.RoleAssistant).TextContent(
			`信息：<1> <> <用户喜欢喝咖啡> <饮食偏好>
信息：<2> <> <用户是软件工程师> <职业>`).Build(), nil
	}

	if contains(lastMsg, "任务") || contains(lastMsg, "trajectory") {
		return message.NewMsg().Role(message.RoleAssistant).TextContent(
			`[{"when_to_use": "数据库查询优化时", "memory": "优先检查索引使用情况"}]`).Build(), nil
	}

	if contains(lastMsg, "工具") || contains(lastMsg, "tool") {
		return message.NewMsg().Role(message.RoleAssistant).TextContent(
			`摘要：成功执行搜索
评价：参数设置合理，结果准确
评分：0.95`).Build(), nil
	}

	return message.NewMsg().Role(message.RoleAssistant).TextContent(" understood").Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func (m *mockModel) ModelName() string { return m.name }

func (m *mockModel) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Chat(ctx, []*message.Msg{msg})
}

func (m *mockModel) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	resp, _ := m.Call(ctx, msg)
	ch <- resp
	close(ch)
	return ch, nil
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || 
		 len(s) > len(substr) && 
		 (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		  findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func main() {
	ctx := context.Background()
	model := &mockModel{name: "mock"}

	fmt.Println("=== AgentScope.Go ReMe 记忆提取器演示 ===")
	fmt.Println()

	// 1. 演示 PersonalSummarizer
	fmt.Println("1. 个人记忆提取示例")
	fmt.Println("------------------------")
	demoPersonalSummarizer(ctx, model)

	fmt.Println()

	// 2. 演示 ProceduralSummarizer
	fmt.Println("2. 程序记忆提取示例")
	fmt.Println("------------------------")
	demoProceduralSummarizer(ctx, model)

	fmt.Println()

	// 3. 演示 ToolSummarizer
	fmt.Println("3. 工具记忆提取示例")
	fmt.Println("------------------------")
	demoToolSummarizer(ctx, model)

	fmt.Println()

	// 4. 演示 MemoryDeduplicator
	fmt.Println("4. 记忆去重示例")
	fmt.Println("------------------------")
	demoDeduplicator(ctx)
}

func demoPersonalSummarizer(ctx context.Context, model *mockModel) {
	summarizer := memory.NewPersonalSummarizer(model, "zh")

	// 模拟对话消息
	messages := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("你好，我是一名软件工程师").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("很高兴认识您！").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("我喜欢在早上喝咖啡").Build(),
	}

	fmt.Println("输入对话:")
	for _, m := range messages {
		fmt.Printf("  %s: %s\n", m.Role, m.GetTextContent())
	}

	// 提取观察
	observations, err := summarizer.ExtractObservations(ctx, messages, "user_alice")
	if err != nil {
		log.Printf("提取观察失败: %v", err)
		return
	}

	fmt.Println("\n提取的观察记忆:")
	for _, obs := range observations {
		fmt.Printf("  - %s (关键词: %s)\n", obs.Content, obs.WhenToUse)
	}
}

func demoProceduralSummarizer(ctx context.Context, model *mockModel) {
	summarizer := memory.NewProceduralSummarizer(model, "zh")

	// 模拟执行轨迹
	trajectory := memory.Trajectory{
		Messages: []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent("优化这个查询").Build(),
			message.NewMsg().Role(message.RoleAssistant).TextContent("正在分析...").Build(),
			message.NewMsg().Role(message.RoleUser).TextContent("查询变快了").Build(),
		},
		Score:    0.95,
		TaskName: "db_optimization",
	}

	fmt.Println("执行轨迹:")
	for _, m := range trajectory.Messages {
		fmt.Printf("  %s: %s\n", m.Role, m.GetTextContent())
	}
	fmt.Printf("成功率: %.2f\n", trajectory.Score)

	// 提取任务记忆
	memories, err := summarizer.ExtractFromSingleTrajectory(ctx, trajectory)
	if err != nil {
		log.Printf("提取任务记忆失败: %v", err)
		return
	}

	fmt.Println("\n提取的程序记忆:")
	for _, mem := range memories {
		fmt.Printf("  - %s\n", mem.Content)
		fmt.Printf("    适用场景: %s\n", mem.WhenToUse)
	}
}

func demoToolSummarizer(ctx context.Context, model *mockModel) {
	summarizer := memory.NewToolSummarizer(model, "zh")

	// 模拟工具调用结果
	results := []memory.ToolCallResult{
		{
			CreateTime: time.Now(),
			ToolName:   "web_search",
			Input:      map[string]any{"query": "Go语言教程", "max_results": 10},
			Output:     "找到10个相关结果",
			TokenCost:  150,
			Success:    true,
			TimeCost:   1.2,
		},
		{
			CreateTime: time.Now(),
			ToolName:   "web_search",
			Input:      map[string]any{"query": "Python教程", "max_results": 10},
			Output:     "找到15个相关结果",
			TokenCost:  180,
			Success:    true,
			TimeCost:   1.5,
		},
	}

	fmt.Println("工具调用历史:")
	for i, r := range results {
		fmt.Printf("  调用 %d: %s (query=%v, success=%v)\n", i+1, r.ToolName, r.Input["query"], r.Success)
	}

	// 评估工具调用
	for i := range results {
		if err := summarizer.EvaluateToolCall(ctx, &results[i]); err != nil {
			log.Printf("评估失败: %v", err)
		}
	}

	fmt.Println("\n评估结果:")
	for i, r := range results {
		fmt.Printf("  调用 %d: 评分=%.2f, 摘要=%s\n", i+1, r.Score, truncate(r.Summary, 30))
	}
}

func demoDeduplicator(ctx context.Context) {
	// 创建一些测试记忆（带重复）
	memories := []*memory.MemoryNode{
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "用户喜欢喝咖啡"),
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "用户喜欢喝咖啡和茶"), // 包含关系
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "用户是一名工程师"),
		memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "用户喜欢喝咖啡"), // 完全重复
	}

	fmt.Println("原始记忆列表:")
	for i, m := range memories {
		fmt.Printf("  %d. %s\n", i+1, m.Content)
	}

	// 使用简单去重
	unique, removed := memory.SimpleDeduplicate(memories, 0.7)

	fmt.Printf("\n去重后 (阈值 0.7):\n")
	fmt.Printf("  保留 %d 条:\n", len(unique))
	for _, m := range unique {
		fmt.Printf("    - %s\n", m.Content)
	}
	fmt.Printf("  移除 %d 条\n", len(removed))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
