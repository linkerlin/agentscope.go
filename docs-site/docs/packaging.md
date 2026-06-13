# 发布包与依赖管理

## Go Module

AgentScope.Go 使用标准 Go module 管理依赖：

```bash
go get github.com/linkerlin/agentscope.go@v2.0.0
```

## 版本策略

遵循 [Semantic Versioning](https://semver.org/)：

- **MAJOR**：不兼容的 API 变更
- **MINOR**：向后兼容的功能添加
- **PATCH**：向后兼容的问题修复

## 发布流程

详见 [RELEASE_CHECKLIST.md](./RELEASE_CHECKLIST.md)。

## Docker 镜像

### 构建

```bash
docker build -t agentscope-go:v2.0.0 .
```

### 运行

```bash
docker run -d \
  -p 8080:8080 \
  -e REDIS_ADDR=redis:6379 \
  -e JWT_SECRET=change-me \
  -e OPENAI_API_KEY=sk-... \
  agentscope-go:v2.0.0
```

### docker-compose 一键启动

```bash
cd docs-site
docker-compose up -d
```

包含：
- Gateway 服务（自动装配）
- Redis 持久化
- 可选 Nginx 反向代理

## Helm Chart（可选）

```bash
helm install agentscope-go ./helm-chart \
  --set redis.enabled=true \
  --set jwt.secret=change-me
```

## 依赖安全

定期运行安全扫描：

```bash
make vulncheck
# 或
govulncheck ./...
```

CI 已集成 `govulncheck` 工作流。
