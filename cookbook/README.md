# AgentScope.Go Cookbook

本目录提供可运行的 recipes，展示如何将 AgentScope.Go 的能力组合成完整解决方案。

---

## Recipes

| Recipe | 路径 | 能力组合 |
|--------|------|----------|
| **长文档摘要** | [`mapreduce_summary/`](mapreduce_summary/) | `workflow.MapReduce` + `agent/react` |
| **多 Agent 审稿** | [`multi_agent_review/`](multi_agent_review/) | `reflection.SelfReflectingAgent` + `pipeline.Pipeline` |
| **RAG 问答** | [`rag_qa/`](rag_qa/) | `loader` + `embedding` + `memory.ReMeVectorMemory` + `agent/react` |
| **定时报告 Agent** | [`scheduled_report/`](scheduled_report/) | `schedule.Scheduler` + `agent/react` |
| **自愈 Agent** | [`self_healing_agent/`](self_healing_agent/) | `evolver.GEPFlow` + remember/recall |

---

## 快速运行

```bash
cd cookbook/mapreduce_summary
export OPENAI_API_KEY=sk-...
go run .
```

---

## 后续计划

- 多模态 RAG（图片 + 文本）
- A2A 多 Agent 协作工作流
- 与 Gateway 集成的生产级 Cookbook
- 实时语音 Agent Pipeline
