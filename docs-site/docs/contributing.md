# Contributing to AgentScope.Go

感谢你对 AgentScope.Go 的兴趣！我们欢迎 Issue、Pull Request、文档改进、示例补充和任何形式的反馈。

## 开发环境

- **Go**：1.25 或更高版本
- **golangci-lint**（可选但推荐）：用于本地 lint
- **Docker**（可选）：用于 Workspace / E2B / 向量数据库集成测试

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/linkerlin/agentscope.go.git
cd agentscope.go

# 构建全部包
go build ./...

# 运行全部测试（含 race 检测）
go test ./... -race -count=1 -timeout=12m

# 格式化与 lint
make fmt
make vet
make lint
```

## 提交 Issue

在创建 Issue 前，请先搜索是否已有相似问题。 Issue 分为以下几类：

- **Bug Report**：描述复现步骤、期望行为、实际行为、环境信息
- **Feature Request**：说明使用场景、期望 API、替代方案
- **RFC**：涉及架构变更、新模块、重大功能，需先开 Issue 讨论

## 提交 Pull Request

1. **Fork 仓库** 并从 `main` 分支创建功能分支：`feat/xxx`、`fix/xxx`、`docs/xxx`、`test/xxx`
2. **保持最小改动**：一个 PR 聚焦一个目标，避免巨型 PR
3. **遵循 Commit Message 规范**：
   - `feat(module): description`
   - `fix(module): description`
   - `docs(module): description`
   - `test(module): description`
   - `refactor(module): description`
   - `chore(module): description`
4. **补充测试**：新增功能必须包含单元测试；修复 bug 必须包含回归测试
5. **更新文档**：如改动影响用户接口，请同步更新 README、`docs/`、`MIGRATION.md`
6. **确保 CI 通过**：`go test ./... -race` 全绿，`gofmt -l .` 为空

## 代码规范

- 使用标准 Go 风格，`gofmt` 格式化
- 优先使用 `context.Context` 传递上下文
- 优先使用 interface 和 struct embedding 实现扩展
- 错误处理显式，避免吞错
- 并发代码必须通过 `-race` 检测
- 新 public API 需要 godoc 注释

## 测试规范

- 单元测试：`*_test.go` 与源码同包
- 集成测试：使用 `//go:build integration` 标签，避免默认运行
- 示例代码：放置于 `examples/` 或 `scripts/`，必须可独立编译运行

## 文档贡献

文档位于 `docs/` 和 `README.md`。改进包括：

- 修复错别字或表述不清
- 补充模型后端使用示例
- 补充部署与运维指南
- 翻译（中文/英文）

## 行为准则

参与本项目即表示你同意遵守 [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)。

## 安全漏洞

如发现安全漏洞，请按 [SECURITY.md](./SECURITY.md) 中的流程私下报告，勿在公开 Issue 中披露。

## 获得帮助

- 阅读 [README.md](./README.md) 和 [docs/](./docs/)
- 查看 [TODO.md](./TODO.md) 了解当前任务
- 在 Issue 中 `@` maintainer

再次感谢你的贡献！
