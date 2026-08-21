# AgentScope.Go v2.6.0 Release Notes

> 🎉 **AgentScope.Go v2.6.0** —— 终端 TUI、Workspace 服务化、治理-演化闭环、多租户会话隔离。本版兑现《演进方案v5》Phase 14 全部路线，AgentScope.Go 从"多平台 Agent 服务"进化为"具备终端调试入口、工作区服务化与治理闭环的生产级平台"。
>
> 基准 2026-08-21 vs Python AgentScope main（0d54503e）。本版之后，Python 最新提交（6 个实质）中的差距已全部补齐；同时 Go 独有的 controlplane 治理平面完成与 evolver 的闭环接线。

---

## 主要更新

### 1. Console 终端 TUI（对标 Python `agentscope.console` #2297）

Agent 可在终端里交互试用与调试（bubbletea + bubbles + lipgloss）：

````
┌ AgentName · console · verbosity default ┐
│ user> 42 乘 17                          │
│ · tool call: calculator                 │
│ human confirmation required:            │
│   - calculator {"a":42,"b":17,...}      │
│ allow 'calculator'? [y]es / [N]o / [a]lways │
│ 42 multiply 17 = 714                    │
└ enter 发送 · ctrl+c 中断 · exit 退出 ┘
````

- **三态机**（idle/running/confirming），ReplyStream 事件经 `waitForEvent` tea.Cmd 桥接，实时流式渲染
- **三档 verbosity**：quiet（仅回复+错误）/ default（+思考/工具/令牌/HITL）/ debug（+生命周期标记）
- **HITL 完整闭环**：逐工具 `[y]es/[N]o/[a]lways` 确认 → `InjectEvent(NewUserConfirmResult)` 恢复；Ctrl+C 拒绝全部
- **运行中中断**：Ctrl+C → `agent.Interrupt()`（类型断言优雅降级）
- `examples/console/`：calculator + ModeDefault 权限完整演示；`console/` 8 测试全绿

### 2. Workspace 服务化（对标 #2187/#2257/#1951/#2283）

工作区从"后端实现"升级为"服务化 API"：

- **Artifact 端点**：`GET /workspace/list_dir` + `GET /workspace/read_file`（只读 + safeJoin 防穿越 + 5MiB 上限）
- **工作目录状态**：`GET /workspace/status`（目录 + git 分支/porcelain 变更，无 git 优雅降级）
- **跨 agent 共享工作区**：复用 `Session.WorkspaceID`（零 schema 改动）——同用户多 agent/session 挂同一命名目录 `<root>/<user>/shared/<name>`（目录+skills+MCP 全共享，路径段消毒防穿越）
- **Skills 隔离与选择**：AddSkill 写入 **agent 级库**（跨会话）+ `POST /workspace/skill/select` 按白名单为会话挑选（未知名称报错且不变更；默认行为向后兼容）

### 3. 治理-演化闭环（Go 独有方向）

controlplane 治理平面 ↔ evolver 自演化打通：

- **auto-solidify**：goal → completed 迁移自动触发 `evolver.Solidify`（DecisionSource=controlplane、PrimaryCause=goal_id、Signals=[goal_completed]），`AppConfig.AutoSolidifyOnGoalComplete` opt-in（默认关）
- 防重复：仅真实状态迁移触发（重复 PATCH 同状态不重复固化）；异步 goroutine + recover 防护（外部 Evolver panic 不击穿服务）
- **steer/打断端点**：`POST /v2/sessions/{id}/steer`（mid-turn 注入用户消息）+ `POST /v2/sessions/{id}/interrupt`（复用 Terminate）
- **web_ui 控制台**：composer 新增"注入/打断"操作行
- `examples/controlplane_demo` 第 10 步：goal 完成 → capsule 固化 → 可列出（实测闭环）

### 4. 多租户会话隔离（安全修复）

**修复真实缺口**：v2 的 POST/GET/DELETE/WS 原先不校验 session 归属——认证后任意用户可订阅/终止他人 run。

- `checkSessionAccess`：storage 可用时校验 session 属主，跨用户一律 **404**（不泄露存在性）；无 storage 单用户模式放行
- 覆盖 4 个入口：Streamable HTTP POST/GET/DELETE + WebSocket 握手前
- 2 组集成测试（SSE 三动词隔离 + WS 握手拒绝）

### 5. 其他改进

- **中断原因保留**（#2209 对齐）：react 中断恢复文案附带被截断的部分回复摘要
- **修复 `MockEvolver.Solidify` panic**：nil Gene（合法输入）不再崩溃
- **示例 LLM 配置统一**（v2.5.1 期间）：全部示例改用 `internal/llmenv`（OPENAI_API_KEY / OPENAI_BASE_URL / OPENAI_MODEL，环境变量优先 + `.env` 回退），新增 `.env.example`
- **文档**：`docs/WORKSPACE.md`（7 后端 + 服务化指南）；AGENTS.md 同步至当期架构（165 包 / ~71,100 非测试行）

---

## 升级与兼容

- 全部新能力 **opt-in**：console/共享工作区/skills 选择/auto-solidify 默认关闭或向后兼容，升级零破坏
- 多租户隔离在配置了 Storage 的部署中**默认生效**——若你的客户端此前跨用户共享 session_id（不受支持的行为），将开始收到 404

## 测试

`go test ./... -race -count=1` 全绿（**93 包**）。本版新增：console 8 + workspace 服务化 5 + auto-solidify 3 + steer/interrupt 4 + 多租户隔离 2 + react 中断 1 = **23 个新测试**。

---

## 仅剩 maintainer 执行

```bash
git tag v2.6.0 && git push --tags
gh release create v2.6.0 -F RELEASE_NOTES_v2.6.0.md
```
