# Contributing to AgentScope Go

感谢你对 AgentScope Go 的贡献！本项目采用地道的 Go 实践构建生产级 AI Agent 框架，追求高测试覆盖（-race 必过）、清晰架构与可维护性。

## 快速开始

1. 确保 Go 1.25+
2. `git clone https://github.com/linkerlin/agentscope.go && cd agentscope.go`
3. `go mod tidy`
4. `make test` （或 `go test -race -count=1 ./...`）

## 提交前检查（强制）

使用根目录 Makefile（推荐）：

```bash
make fmt          # 格式化
make fmt-check    # 验证格式（必须通过）
make vet
make build
make test         # -race 全量，必须绿
```

或者手动：
- `gofmt -l .` 必须为空
- `go test ./... -race -count=1` 全绿
- CI（GitHub Actions）会自动运行格式检查 + vet + 构建 + race 测试 + golangci-lint

## 编码规范

详见 [AGENTS.md](AGENTS.md) 中的「编码规范」和「关键设计决策」章节。

重要原则（摘要）：
- 事件驱动（`ReplyStream` 返回 `<-chan event.AgentEvent`）
- struct embedding 复用 `agent.Base`
- 并发用 errgroup + atomic 标志
- 新 embedding 代码请使用顶级 `embedding/` 包
- 工具使用 `tool.Response` 和规范接口
- memory 内部的 `EmbeddingModel`（float32 单/批量）与顶级 `model.EmbeddingModel` 保持区分

## 提 PR 流程

1. Fork + 创建特性分支
2. 实现 + 添加/更新测试（优先使用 table test + -race）
3. 本地通过 `make ci`（或等价检查）
4. 提交时包含清晰描述，引用相关 issue（如有）
5. PR 会触发 CI，所有 job 必须通过

## 文档与示例

- 更新 README / docs/ 时请保持中英一致（核心文档以中文为主）
- 新功能请同时更新对应 example（推荐放在 `examples/` 下）
- 大型变更请先在 issue 或讨论中对齐设计

## 问题与讨论

- Bug / 特性请求：使用 GitHub Issues
- 设计讨论：可在 issue 中使用 "design" 标签
- 安全问题：请私下联系维护者

## 许可证

贡献的代码将以 Apache-2.0 许可证发布（与项目一致）。

再次感谢！我们欢迎任何形式的贡献（代码、文档、测试、benchmark、bug report）。

---

参考：
- [AGENTS.md](AGENTS.md) — 开发备忘录与架构决策
- [项目全面审阅报告.md](项目全面审阅报告.md) — 持续改进记录
- Makefile 根目标帮助 `make help`