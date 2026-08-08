# Plugin 系统

AgentScope.Go 的 Plugin 系统让第三方扩展以统一生命周期接入框架：
**Init → Register → Shutdown**。支持静态编译插件与 Linux `.so` 动态加载。

## 插件接口

```go
type Plugin interface {
    Name() string                    // 唯一标识（必须匹配 YAML 中的 name）
    Init(config PluginConfig) error  // 配置注入（params 从 YAML 解析）
    Register(r *Registrar) error     // 注册扩展点
    Shutdown() error                 // 资源清理（优雅关闭时调用）
}
```

## YAML 配置

```yaml
plugins:
  - name: echo-plugin      # 匹配 Plugin.Name()
    type: echo-plugin      # factory 注册的类型名
    enabled: true          # false 则跳过
    priority: 10           # 初始化顺序（小者先）
    params:                # 自定义参数 → Init 的 PluginConfig.Params
      prefix: "[echo] "
```

加载：`plugin.LoadConfigFile(path)` → `mgr.LoadConfig(cfg)`。

## Manager 生命周期

```go
mgr := plugin.NewManager()
mgr.RegisterFactory("echo-plugin", func() plugin.Plugin { return echoplugin.New("[echo] ") })

cfg, _ := plugin.LoadConfigFile("plugins.yaml")
mgr.LoadConfig(*cfg)    // 解析 YAML → 按 name/type/enabled 装配插件
mgr.InitAll()           // 逐插件 Init（按 priority 升序）
mgr.RegisterAll(reg)    // 逐插件 Register（向 Registrar 注册扩展点）
mgr.ShutdownAll(ctx)    // 逐插件 Shutdown（逆序清理）
```

## Registrar 注册点

| 方法 | 用途 |
|------|------|
| `RegisterTool(name, tool)` | 注册工具（需 `AddToolRegistrar` 桥接到 toolkit） |
| `RegisterHook(name, hook)` | 注册 Agent 钩子 |
| `RegisterMiddleware(name, mw)` | 注册中间件 |
| `CreateModel/Formatter/Memory(type, params)` | 通过工厂创建模型/格式化器/记忆 |
| `Available*Types()` | 查询可用工厂类型 |
| `Registered*()` | 查询已注册项 |

## .so 动态加载（Linux）

`plugin/loader_linux.go` 支持从 `.so` 文件运行时加载插件（build tag 隔离，
其他平台返回不支持）。用于热插拔部署；跨平台部署用静态编译插件。

## 示例

```bash
go run ./examples/plugin_demo/
```

一个 `echo` 插件：YAML 装配 → Init 读 prefix → Register 注册 echo 工具 →
执行 → 优雅关闭。完整代码见 `examples/plugin_demo/`（echoplugin 包 + main + plugins.yaml）。
