# AgentScope Go Console — 现代 Web UI

零构建依赖的单页控制台：原生 ES5 JS + SSE + `go:embed`，**无需 npm/Node 工具链**，单二进制部署。
对标 Python AgentScope 的 React Web UI，用最轻的方式提供等价的交互体验。

## 运行

```bash
cd examples/web_ui
# Demo 模式（内置 demo agent，无需 API Key）
go run .

# 真实模型模式
DASHSCOPE_API_KEY=sk-... go run .
```

打开 http://localhost:8080

## 功能面板

### 💬 Chat
- AG-UI 协议 SSE 流式（POST/GET `/v2/chat?protocol=agui`）
- 实时展示：文本增量、推理过程（可折叠）、工具调用卡片、HITL 提示
- 页面加载自动重连（session id 存 localStorage）

### 📚 Knowledge Bases（Phase 5 托管 KB）
- 知识库 CRUD（`/api/v1/knowledge-bases`）
- 文档上传（multipart，自动解析→分块→嵌入→索引）
- 语义检索（`/api/v1/knowledge-bases/{id}/search`）
- 文档列表与删除
- Demo 用确定性 stub embedder 离线运行；生产替换 `buildKBService()` 中的 embedder

### ⚙ System
- Health 检查
- 模型列表（`/api/v1/models`）

## 架构

```
examples/web_ui/
├── main.go          # 服务入口：注册路由 + KBService + embed 静态资源
├── agent.go         # Agent 构建（DashScope / demo）
├── static/
│   ├── index.html   # 控制台外壳（侧边栏 + 视图区）
│   ├── styles.css   # 现代 GitHub 风深色主题
│   └── app.js       # 全部前端逻辑（路由 / AG-UI chat / KB CRUD / system）
```

- **零构建**：手写 vanilla JS，无框架、无打包器，浏览器直接加载
- **单二进制**：`go:embed static/*` 内嵌所有资源
- **API 直连**：前端直接调用 gateway HTTP API，无中间代理层

## 自定义

替换 embedder 为真实模型：

```go
// main.go buildKBService()
mgr := kb.NewCollectionPerKBManager(store, func(string) (kb.Embedder, error) {
    return embedding.NewOpenAI(apiKey, "text-embedding-3-small"), nil
})
```

扩展面板：在 `index.html` 加 `<button class="nav-item" data-view="xxx">` + `<section id="view-xxx">`，
在 `app.js` 的路由与对应 section 实现视图逻辑。

## 与 Python Studio 互操作

Go gateway 的 AG-UI 协议与 Python `examples/web_ui` 兼容，可将 Python React 前端连到 Go 后端：
启动 Go gateway（暴露 `/v2/chat`），指向 Python 前端的 API 地址即可。
