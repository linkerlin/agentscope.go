# Workspace 沙箱与工作区服务（Workspace）

AgentScope.Go 用统一的 `workspace.Workspace` 接口抽象"Agent 能碰到的文件系统与执行环境"，
7 个后端覆盖从本地到云端、从轻量隔离到容器编排的部署场景。本文档说明各后端用法、
共享工作区、skills 选择与 gateway 工作区 HTTP 端点。

## 核心接口

```go
type Workspace interface {
    ID() string
    ReadFile(ctx context.Context, path string) ([]byte, error)
    WriteFile(ctx context.Context, path string, data []byte) error
    ListDir(ctx context.Context, path string) ([]string, error)
    Stat(ctx context.Context, path string) (*FileInfo, error)
    Execute(ctx context.Context, command string, args ...string) (*ExecResult, error)
    Close() error
}
```

## 7 个后端

| 后端 | 构造入口 | 适用场景 | 平台 |
|------|----------|----------|------|
| **Local** | `NewLocalWorkspace(id, baseDir)` | 本地文件系统，最简调试 | 全平台 |
| **Docker** | `NewDockerWorkspace(id, containerID)` | 容器隔离，标准 CI/生产 | 需 docker |
| **E2B** | `NewE2BWorkspace(id, sandboxID, client)` | 云沙箱（E2B.io） | 需 API key |
| **K8s** | `NewK8sWorkspace(ctx, cfg)` | Pod 生命周期（kubectl CLI，无 client-go 重依赖） | 需 kubectl |
| **Bubblewrap** | `NewBubblewrapWorkspace(cfg)` | Linux 用户命名空间轻量沙箱（零 root） | Linux |
| **Daytona** | `NewDaytonaWorkspace(ctx, cfg)` | Daytona 云开发环境（REST API） | 需 token |
| **OpenSandbox** | `NewOpenSandboxWorkspace(ctx, cfg)` | OpenSandbox 远程沙箱（REST API） | 需 token |

### 最小示例

```go
ws := workspace.NewLocalWorkspace("demo", "./sandbox")
defer ws.Close()

err := ws.WriteFile(context.Background(), "hello.txt", []byte("hi"))
res, err := ws.Execute(context.Background(), "cat", "hello.txt")
fmt.Println(res.Stdout) // hi
```

### 隔离级别选择

- **开发/调试**：Local（无隔离）→ Bubblewrap（Linux 轻量）→ Docker
- **生产多租户**：Docker / K8s（Pod 隔离 + 资源限制）
- **云端执行**：E2B / Daytona / OpenSandbox（代码不在本机落地）

K8s 支持包装已存在的 Pod（`NewK8sWorkspaceForExistingPod`）；Bubblewrap 支持
自定义只读挂载、网络隔离（`--unshare-net`）与 tmpfs 大小。

## 会话工作区（gateway 服务化）

gateway 的 `WorkspaceManager` 按 `(user, agent, session)` 解析每个会话的工作区目录
`<root>/<user>/<agent>/<session>`，并附加 skills 与 MCP 注册：

```go
wm := gateway.NewWorkspaceManager("./workspaces", "./skills")
srv := gateway.NewServer(agent).WithWorkspaceManager(wm)
srv.RegisterWorkspaceRoutes()
```

### HTTP 端点

| 端点 | 说明 |
|------|------|
| `GET /workspace/list_dir?agent_id=&session_id=&path=` | 列目录（只读） |
| `GET /workspace/read_file?agent_id=&session_id=&path=` | 读文件（只读，5MiB 上限） |
| `GET /workspace/status?agent_id=&session_id=` | 工作目录 + git 分支/变更（无 git 优雅降级） |
| `GET /workspace/agent_skills?agent_id=` | agent 级 skill 库 |
| `POST /workspace/skill/select` `{"names":[...]}` | 按白名单选择会话 skills |
| `GET/POST /workspace/mcp`、`/workspace/skill` | MCP / skill 管理（原有） |

所有文件端点只读并强制 `safeJoin` 路径防护（拒绝绝对路径与 `..` 逃逸）。

### 跨 agent 共享工作区

会话设置 `WorkspaceID` 后即挂到命名共享目录 `<root>/<user>/shared/<name>`，
不同 agent/session 复用同一目录（含 skills 与 MCP 注册，全部共享）：

```go
storage.SaveSession(ctx, &service.Session{
    ID: "s1", UserID: "u1", AgentID: "agentA", WorkspaceID: "team-ws",
})
// agentA 与 agentB 的会话都指向 u1/shared/team-ws
```

### skills 隔离与选择

- `POST /workspace/skill` 添加 skill 时同时写入 **agent 级库**（跨会话可见）
- 新会话默认空选择；`POST /workspace/skill/select` 从 agent 库挑选（未知名称报错且不变更）

## 相关示例

- `examples/multi_tenant_workspace/`：会话工作区 + 权限 + 工具卸载的完整服务
- `examples/web_ui/`：内置工作区 + 会话文件/steer 端点接入的控制台
- `examples/console/`：终端 TUI（可配合工具确认演示 HITL）

## 测试

```bash
go test ./workspace/ -race -count=1
go test ./gateway/ -run TestWorkspace -race -count=1
```