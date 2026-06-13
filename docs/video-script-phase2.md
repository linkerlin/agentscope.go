# AgentScope.Go Phase 2 视频脚本大纲

> 时长：约 5 分钟  
> 目标观众：Go 开发者、AI Agent 工程师  
> 用途：快速展示 Phase 2 新增的 Studio 面板、Cookbook 与模型示例

---

## 中文脚本大纲

### 0. 开场（0:00-0:30）
- 标题卡：AgentScope.Go Phase 2 — Studio、Cookbook、模型示例全面升级
- 讲师自我介绍（可选）
- 一句话价值主张：AgentScope.Go 不仅是 Python 版的高性能替代，更是面向云原生 Agent 服务的 Go 框架

### 1. 5 分钟跑通第一个 Agent（0:30-1:30）
- 克隆仓库：`git clone ...`
- 设置 `OPENAI_API_KEY`
- 启动 Studio：`cd examples/studio && go run .`
- 浏览器打开 `http://localhost:8081`
- Demo Register → 创建 Agent → 进入 Chat 发送消息
- 强调：SSE 流式、自动工具调用、Workspace 隔离

### 2. 模型示例脚本库（1:30-2:30）
- 切换到 `scripts/model_examples/`
- 演示 `openai_chat_call`、`openai_chat_stream`、`openai_chat_multimodal`
- 展示 Anthropic / DashScope / DeepSeek / Ollama 示例
- 强调：每家后端一个脚本，快速验证连通性

### 3. Cookbook 实战（2:30-3:30）
- 展示 `cookbook/mapreduce_summary`：长文档摘要
- 展示 `cookbook/rag_qa`：Loader + Embedding + ReMe 检索问答
- 展示 `cookbook/self_healing_agent`：GEP 自演化
- 强调：从示例到生产方案的桥梁

### 4. Studio 新面板巡礼（3:30-4:30）
- `/models`：模型卡片可视化
- `/schedules`：定时任务自动刷新列表
- `/memory`：ReMe 向量记忆调试（add + semantic search）
- `/a2a`：A2A Registry 浏览器（discover / register / health check）
- `/evolver`：Gene / Capsule 面板与 GEP dry-run

### 5. 结尾与下一步（4:30-5:00）
- 总结 Phase 2 成果
- GitHub 仓库链接、文档站点（即将上线）
- 预告 Phase 3：向量数据库、SSO/RBAC、Langfuse 集成
- CTA：Star、提 Issue、贡献代码

---

## English Script Outline

### 0. Intro (0:00-0:30)
- Title card: AgentScope.Go Phase 2 — Studio, Cookbook & Model Examples
- One-liner value prop: AgentScope.Go is not just a high-performance alternative to the Python version, but a Go-native framework for cloud-native agent services.

### 1. First Agent in 5 Minutes (0:30-1:30)
- Clone repo, set `OPENAI_API_KEY`
- Run Studio: `cd examples/studio && go run .`
- Open browser at `http://localhost:8081`
- Demo Register → Create Agent → Chat
- Highlight SSE streaming, auto tool calls, workspace isolation

### 2. Model Examples (1:30-2:30)
- Walk through `scripts/model_examples/`
- Demo `openai_chat_call`, `openai_chat_stream`, `openai_chat_multimodal`
- Show Anthropic / DashScope / DeepSeek / Ollama examples
- Emphasize one script per provider for quick connectivity checks

### 3. Cookbook Recipes (2:30-3:30)
- `cookbook/mapreduce_summary`: long-document summarization
- `cookbook/rag_qa`: Loader + Embedding + ReMe retrieval Q&A
- `cookbook/self_healing_agent`: GEP self-evolution
- Emphasize the bridge from examples to production recipes

### 4. Studio New Panels Tour (3:30-4:30)
- `/models`: visual model card browser
- `/schedules`: auto-refreshing scheduled task list
- `/memory`: ReMe vector memory debug panel (add + semantic search)
- `/a2a`: A2A Registry browser (discover / register / health check)
- `/evolver`: Gene / Capsule panel with GEP dry-run

### 5. Outro & Next Steps (4:30-5:00)
- Recap Phase 2 achievements
- GitHub link and upcoming docs site
- Preview Phase 3: vector DB connectors, SSO/RBAC, Langfuse
- CTA: Star, open issues, contribute

---

## 录制检查清单

- [ ] 环境已准备好 `OPENAI_API_KEY`
- [ ] 本地已启动 Ollama（用于本地模型示例）
- [ ] 屏幕录制分辨率 1920x1080
- [ ] 终端字体足够大
- [ ] 浏览器缩放 125% 便于观众阅读
- [ ] 关键操作有 1-2 秒停顿
- [ ] 中文旁白 + 英文字幕（或反之）
