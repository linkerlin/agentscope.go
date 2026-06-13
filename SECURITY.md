# Security Policy

## 支持的版本

| 版本 | 支持状态 |
|------|----------|
| v2.0.x | ✅  actively supported |
| v2.0.0-rc.x | ⚠️  best-effort, upgrade recommended |
| v1.x | ❌  no longer supported |

## 报告安全漏洞

如果你发现了 AgentScope.Go 中的安全漏洞，请通过以下方式私下报告：

- **GitHub Security Advisories**：在仓库页面选择 "Report a vulnerability"
- **Email**：`security@agentscope-go.io`（如可用）

请不要在公开的 Issue、Discussion 或 PR 中披露漏洞细节。

## 报告内容

请尽可能提供以下信息：

1. 漏洞描述与影响范围
2. 复现步骤或 PoC（最小可复现示例）
3. 受影响的版本
4. 建议的修复方案（如有）
5. 你的联系方式（可选）

## 处理流程

1. **确认**：维护者将在 5 个工作日内确认收到报告
2. **评估**：评估漏洞严重性和影响范围
3. **修复**：在私有分支中开发修复，准备安全补丁
4. **披露**：修复后发布安全公告（Security Advisory）和补丁版本
5. **致谢**：在公告中感谢报告者（如报告者同意）

## 安全最佳实践

使用 AgentScope.Go 时，建议遵循以下安全实践：

- 不要将 API Key、Credential 明文提交到代码仓库
- 使用 `service.Cipher` 加密持久化凭证
- 生产环境使用 Redis + TLS，避免使用 MemoryStorage
- 启用 `PermissionEngine` 限制工具执行范围
- 使用 `Workspace`（Docker/E2B）隔离不可信代码执行
- 定期更新依赖：`go get -u ./...` 并运行 `govulncheck`
- 为 Gateway 启用 HTTPS 和身份验证（JWT/API Key）

## 已知漏洞

暂无公开已知漏洞。历史安全公告见 [GitHub Security Advisories](https://github.com/linkerlin/agentscope.go/security/advisories)。
