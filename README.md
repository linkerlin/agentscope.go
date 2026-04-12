# agentscope.go

A Go implementation of [AgentScope Java](https://github.com/agentscope-ai/agentscope-java) — a production-ready AI agent framework for building LLM-powered applications in Go.

## Overview

AgentScope Go provides everything you need to create intelligent agents using the ReAct (Reasoning + Acting) paradigm: tool calling, memory management, multi-agent collaboration, and more — all implemented idiomatically in Go.

## Quick Start

**Requirements:** Go 1.22+

```bash
go get github.com/linkerlin/agentscope.go
```

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    chatModel, _ := openai.Builder().
        APIKey(os.Getenv("OPENAI_API_KEY")).
        ModelName("gpt-4o-mini").
        Build()

    agent, _ := react.Builder().
        Name("Assistant").
        SysPrompt("You are a helpful AI assistant.").
        Model(chatModel).
        Build()

    response, _ := agent.Call(context.Background(), message.NewMsg().
        Role(message.RoleUser).
        TextContent("Hello! What can you help me with?").
        Build())

    fmt.Println(response.GetTextContent())
}
```

## Supported Models

| Provider       | Package                                        |
|---------------|------------------------------------------------|
| OpenAI         | `github.com/linkerlin/agentscope.go/model/openai`      |
| DashScope (Alibaba) | `github.com/linkerlin/agentscope.go/model/dashscope` |

Any OpenAI-compatible endpoint is supported via `BaseURL`.

## Core Packages

| Package | Description |
|---------|-------------|
| `message` | `Msg` type with multimodal content blocks (text, image, audio, video, tool use/result, thinking) |
| `model` | `ChatModel` interface with streaming support |
| `agent` | `Agent` interface |
| `agent/react` | ReAct agent implementation |
| `memory` | `Memory` interface + in-memory implementation |
| `tool` | `Tool` interface + `FunctionTool` adapter |
| `session` | Session management |
| `hook` | Hook system for human-in-the-loop control |
| `plan` | PlanNotebook for structured multi-step task management |

## Using Tools

```go
import "github.com/linkerlin/agentscope.go/tool"

myTool := tool.NewFunctionTool(
    "weather",
    "Get the current weather for a city",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "city": map[string]any{"type": "string"},
        },
        "required": []string{"city"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        city := input["city"].(string)
        return fmt.Sprintf("Sunny, 22°C in %s", city), nil
    },
)

agent, _ := react.Builder().
    Name("WeatherBot").
    Model(chatModel).
    Tools(myTool).
    Build()
```

## Memory

```go
import "github.com/linkerlin/agentscope.go/memory"

mem := memory.NewInMemoryMemory()
agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Memory(mem).
    Build()
```

## Hooks (Human-in-the-Loop)

```go
import "github.com/linkerlin/agentscope.go/hook"

loggingHook := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
    fmt.Printf("[%s] Agent: %s\n", hCtx.Point, hCtx.AgentName)
    return nil, nil
})

agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Hooks(loggingHook).
    Build()
```

## PlanNotebook

```go
import "github.com/linkerlin/agentscope.go/plan"

notebook := plan.NewPlanNotebook()
p := notebook.CreatePlan("Research Task")
notebook.AddStep(p.ID, "Search for information")
notebook.AddStep(p.ID, "Summarize findings")

// Use as a tool in an agent
agent, _ := react.Builder().
    Name("Planner").
    Model(chatModel).
    Tools(notebook.AsTool()).
    Build()
```

## DashScope (Alibaba Cloud)

```go
import "github.com/linkerlin/agentscope.go/model/dashscope"

chatModel, _ := dashscope.Builder().
    APIKey(os.Getenv("DASHSCOPE_API_KEY")).
    ModelName("qwen-max").
    Build()
```

## Examples

- [`examples/hello`](examples/hello/main.go) — Basic agent usage
- [`examples/tools`](examples/tools/main.go) — Agent with calculator tool

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
