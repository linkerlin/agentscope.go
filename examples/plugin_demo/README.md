# plugin_demo — 插件系统示例

演示 AgentScope.Go 的 Plugin 系统：**Init → Register → Shutdown** 三阶段生命周期 +
YAML 配置驱动 + Registrar 注册工具。

## 运行

```bash
go run ./examples/plugin_demo/
```

输出：插件加载（从 plugins.yaml）→ 注册 `echo` 工具 → 执行 → 优雅关闭。

## 插件接口（三阶段生命周期）

```go
type Plugin interface {
    Name() string                       // 唯一标识
    Init(config PluginConfig) error     // 配置注入（params 从 YAML 解析）
    Register(r *Registrar) error        // 注册扩展点（工具/钩子/中间件/模型/记忆）
    Shutdown() error                    // 资源清理
}
```

## YAML 配置

```yaml
plugins:
  - name: echo-plugin      # 匹配 Plugin.Name()
    type: echo-plugin      # factory 注册的类型名
    enabled: true
    priority: 10           # 初始化顺序（小者先）
    params:
      prefix: "[echo] "    # 自定义参数 → Init 的 PluginConfig.Params
```

## Manager 生命周期

```go
mgr := plugin.NewManager()
mgr.RegisterFactory("echo-plugin", func() plugin.Plugin { return echoplugin.New("[echo] ") })

cfg, _ := plugin.LoadConfigFile("plugins.yaml")
mgr.LoadConfig(*cfg)   // 解析 YAML → 按 name/type/enabled 装配
mgr.InitAll()          // 逐插件 Init（按 priority）
mgr.RegisterAll(reg)   // 逐插件 Register（向 Registrar 注册扩展点）
mgr.ShutdownAll(ctx)   // 逐插件 Shutdown（逆序）
```

## Registrar 注册点

| 方法 | 用途 |
|------|------|
| `RegisterTool(name, tool)` | 注册工具（需 `AddToolRegistrar` 桥接 toolkit） |
| `RegisterHook(name, hook)` | 注册钩子 |
| `RegisterMiddleware(name, mw)` | 注册中间件 |
| `CreateModel/Formatter/Memory(type, params)` | 工厂创建（模型/格式化器/记忆） |
| `RegisteredTools/Hooks/...()` | 查询已注册项 |

## 生产用法：.so 动态加载

`plugin/loader_linux.go` 支持 Linux 下从 `.so` 文件动态加载插件（build tag 隔离）。
静态编译插件（如本示例）用于跨平台；`.so` 用于运行时热插拔。
