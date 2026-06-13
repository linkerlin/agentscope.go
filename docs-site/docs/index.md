# AgentScope.Go 文档

欢迎使用 AgentScope.Go 官方文档站点！

## 什么是 AgentScope.Go？

AgentScope.Go 是 [AgentScope](https://github.com/agentscope-ai/agentscope) 的 Go 语言实现 —— 一个生产级的 AI Agent 开发框架，助你使用 Go 构建基于大语言模型的智能应用。

## 核心特性

- **ReAct 范式**：推理 + 行动 + 工具调用
- **事件驱动 V2**：channel 驱动的真事件流，20+ 细粒度事件类型
- **ReMe 长期记忆**：文件型 + 向量型 + Hybrid Search + 5+ 向量后端
- **A2A 协议**：完整实现 + Redis 分布式注册中心 + 一致哈希分片
- **GEP 自演化**：Gene/Capsule/Skill2GEP 对齐前沿自演化引擎
- **生产级服务**：`gateway.NewApp` 一键装配，多租户认证，Session 持久化

## 快速导航

| 文档 | 说明 |
|------|------|
| [5 分钟上手](quickstart.md) | 安装、第一个 Agent、运行 |
| [教程](tutorial.md) | 从 Hello 到生产部署的 5 步教程 |
| [事件流](concepts/event-stream.md) | 理解 V2 事件驱动架构 |
| [ReMe 记忆](advanced/reme.md) | 长期记忆系统深度教程 |
| [A2A 协议](advanced/a2a.md) | Agent 间通信协议指南 |
| [GEP 自演化](advanced/evolver.md) | Gene Evolution Protocol 教程 |
| [部署指南](deployment.md) | Docker、K8s、systemd 部署 |

## 版本信息

- **当前版本**：v2.0.0
- **Go 版本要求**：1.25+
- **许可证**：Apache 2.0

## 获取帮助

- [GitHub Issues](https://github.com/linkerlin/agentscope.go/issues)
- [GitHub Discussions](https://github.com/linkerlin/agentscope.go/discussions)
