# TODO

仅列尚未完成的工作。验收标准在 [演进方案.md](演进方案.md) 的同名 ID；合入后从两份文件同时删除，不保留完成记录。实施顺序以依赖为准，不以历史 ID 的数字为准。2026-10-03 复评后的顺序就是下面各节的顺序。

## 进入 Phase 21 之前

- [ ] **24.2** 版本切割：`[Unreleased]` 收成 v2.8.0（Phase 18）与 v2.9.0（Phase 19 与 20）；`version.go`、发行说明与 annotated tag 对齐
- [ ] **24.3** 停止跟踪根目录 `web_ui` Mach-O，并在 `.gitignore` 忽略该构建产物
- [ ] **24.4** 治理自动记账与 `ShouldRunTurn` 票据对齐；enforcement 打开时不得只写回、不花费
- [ ] **24.5** 一家供应商（DashScope 或 OpenAI）的脱敏 realtime 会话夹具，断言帧名与序号
- [ ] **24.6** `sessionapi` 直接测试：resume 409、请求体 413、steer/interrupt 跨用户 404

## P2 · v3.0 平台收敛

- [ ] **21.6** `ChatService`：第一刀移出治理与 workspace HTTP；第二刀统一 turn runner。两刀分开验收
- [ ] **21.5** Team `Mode=peer`：租约领取不同 todo；CAS 只认 open；同目标不同载荷不得覆盖
- [ ] **21.4** 性能门禁：假模型一轮对话、租约获取与续租、播放队列入队。超过经确认的阈值即失败
- [ ] **21.2** 全功能 TUI：语音、AskUser 表单、工具分组在同一终端，走 21.6 的 runner
- [ ] **21.3** 本地 Gene 仓库：无外部 MCP 可 Run / Reflect / Solidify；崩溃后可继续；然后才在 UI 列出
- [ ] **21.7** v3 API 收敛：删除清单（记忆别名、Studio）；`Call` 保留并写明是事件流收集器
- [ ] **21.8** 非 Linux 插件：官方进程内注册路径与完整示例，`.so` 继续限定 Linux

## 不做

Apple Container、React / npm 重写内置 UI、浏览器内第二套语音 UI、第五个 realtime 供应商、ONNX / LightGBM 路由、把治理强耦合进 `agent/` 内核、ES / Mongo / S3 / NATS / K8s Operator（含补齐已删除的 ES / pgvector 占位）、删除 `Call`、复刻全部模型卡、为抬覆盖率补无行为断言的测试、全仓库微基准进入 CI 阻断。
