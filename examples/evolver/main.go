package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/evolver"
	"github.com/linkerlin/agentscope.go/skill"
)

// Example: 使用 agentscope.go 的 evolver 包对齐 ./evolver/ (GEP) 的优势功能。
// 演示：
// - Gene/Capsule 类型 + 验证 (compact strategy vs ad-hoc skill)
// - MockEvolver + GEPFlow (run -> reflect -> solidify 闭环)
// - Recording 追踪
// - Skill -> Gene 蒸馏 (skillDistiller 风格)
// - 结合 ReMe 风格的 remember/recall (typed memory for genes/capsules)
// - 会议与任务 stub
//
// 真实部署时，将 Evolver client 后端换成通过 gateway MCP 网关暴露的 evolver MCP server
// (agentscope.go 的 session_mcp_gateway 已支持将任意 MCP 工具暴露给 session agent)。
// 这样你的 Agent 即可在运行中自主调用 evolver_run / solidify 等，实现自进化。
func main() {
	fmt.Println("=== AgentScope Go + Evolver GEP Alignment Demo ===")
	fmt.Println("优势对齐要点：Genes(策略基因) > ad-hoc Skills；run/reflect/solidify pipeline；typed memoryGraph/narrative；ATP tasks；safety+audit。")

	ctx := context.Background()

	// 1. 使用 Mock（生产可替换为真实 backed client）
	mock := evolver.NewMockEvolver()
	rec := evolver.NewRecordingEvolver(mock) // 类似 Phase5 TracingMiddlewareAdapter
	flow := evolver.NewGEPFlow(rec)

	// 2. 触发一次演化：模拟 agent 遇到 recurring timeout
	runCfg := evolver.RunConfig{
		Context:  "网关超时 gateway_timeout 导致大 payload 任务失败，重复出现",
		Strategy: "repair-only",
	}
	runRes, solRes, err := flow.RunAndSolidify(ctx, runCfg, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Run: signals=%v gene=%s\nGEPPrompt head: %s...\n", runRes.Signals, runRes.SelectedGene.ID, truncate(runRes.GEPPrompt, 120))
	fmt.Printf("Solidify: ok=%v capsule=%s dry=%v\n", solRes.OK, solRes.CapsuleID, solRes.DryRun)

	// 3. 记录/召回演化记忆（对齐 evolver remember/recall + memoryGraph）
	_ = rec.Remember(ctx, evolver.RememberRequest{
		Text:       "使用 gene_gep_repair_from_errors 成功恢复 timeout，blast 很小",
		Type:       "capsule",
		Category:   evolver.CategoryRepair,
		Importance: 0.92,
		Scope:      "agentscope-go-demo",
	})
	hits, _ := rec.Recall(ctx, evolver.RecallRequest{Query: "timeout", Limit: 3, Category: "capsule"})
	fmt.Printf("Recall hits: %d (narrative/memory graph style retrieval)\n", len(hits))

	// 4. Skill 蒸馏为 Gene（对齐 skill2gep / skillDistiller / skillPublisher）
	sk := &skill.AgentSkill{
		Name:         "timeout_recovery",
		Description:  "处理 gateway timeout：先重试一次，失败则拆分成并行子任务",
		SkillContent: "detect timeout signal\nretry verbatim once\nif still timeout: split work to subagents and merge",
	}
	geneFromSkill := sk.DistillToGene(evolver.CategoryRepair)
	fmt.Printf("Distilled gene from skill: id=%s category=%s signals=%v\n", geneFromSkill.ID, geneFromSkill.Category, geneFromSkill.SignalsMatch)

	// 可选：upsert 回 catalog
	_ = rec.UpsertGene(ctx, geneFromSkill)

	// 5. 会议 stub + 统计（对齐 meeting + stats + safety）
	meet, _ := rec.MeetingStart(ctx, evolver.MeetingStartRequest{Type: "debug", Task: "evolve timeout handling"})
	status, _ := rec.MeetingStatus(ctx, meet.ID)
	st, _ := rec.Stats(ctx)
	safe, _ := rec.SafetyStatus(ctx)
	fmt.Printf("Meeting: %s stage=%s\nStats: %+v\nSafety: %+v\n", meet.ID, status.Stage, st, safe)

	// 6. 展示录制调用（对齐 Phase5 RecordingTracer 模式）
	fmt.Printf("Recorded Evolver calls: %v\n", rec.Calls)

	fmt.Println("\n下一步：在真实 evolver MCP 后端 + gateway 暴露工具后，Agent 运行时可自主完成 GEP 闭环 + 资产固化 + 回滚审计。")
	fmt.Println("See evolver/ package, examples/evolver, and updated docs for full integration guide.")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
