# AgentScope.Go 快速上手

> 5 分钟内运行你的第一个 Agent

## 环境要求

- **Go 1.25** 或更高版本
- 一个 LLM API Key（OpenAI / Anthropic / Gemini 等）

## 安装

```bash
go get github.com/linkerlin/agentscope.go
```

## 第一个 Agent

创建 `main.go`：

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    // 1. 创建模型
    chatModel, _ := openai.Builder().
        APIKey(os.Getenv("OPENAI_API_KEY")).
        ModelName("gpt-4o-mini").
        Build()

    // 2. 创建 Agent
    agent, _ := react.Builder().
        Name("Assistant").
        SysPrompt("You are a helpful AI assistant.").
        Model(chatModel).
        Build()

    // 3. 调用
    response, _ := agent.Call(context.Background(), message.NewMsg().
        Role(message.RoleUser).
        TextContent("Hello! What can you help me with?").
        Build())

    fmt.Println(response.GetTextContent())
}
```

运行：

```bash
export OPENAI_API_KEY=sk-...
go run main.go
```

## 下一步

- [教程](tutorial.md) — 深入学习工具、事件流、多 Agent 编排
- [生产部署](deployment.md) — 将 Agent 部署为服务
- [ONNX 快速开始](ONNX.md) — 图像/音频嵌入与跨模态推理

---

## ONNX 快速开始

AgentScope.Go 支持 ONNX 本地多模态推理（CLIP 图像嵌入 + Whisper 音频嵌入），采用 HTTP 代理方案，零 CGO 依赖。

### 图像嵌入（CLIP）

```go
package main

import (
    "fmt"
    "github.com/linkerlin/agentscope.go/embedding/onnx"
)

func main() {
    // 图像预处理（CLIP）→ 输出 NCHW [1,3,224,224]
    preprocessor := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())
    // vec 需传入 image.Image 接口

    // CLIP 图像嵌入器（HTTP 代理连接 ONNX Runtime 服务）
    clip := onnx.NewCLIPImageEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
    // embedding, _ := clip.EmbedImage(vec)
    fmt.Println("CLIP embedder ready")
}
```

### 音频嵌入（Whisper）

```go
import "github.com/linkerlin/agentscope.go/embedding/onnx"

// 音频预处理（Whisper）→ 输出 Mel 频谱图 [1,80,3000]
audioProc := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())
// mel, _ := audioProc.Preprocess(pcmSamples, 16000)

// Whisper 音频嵌入器
whisper := onnx.NewWhisperAudioEmbedder(onnx.DefaultWhisperAudioEmbedderConfig())
// embedding, _ := whisper.EmbedAudio(mel)
```

运行完整示例：

```bash
cd examples/onnx
go run main.go
```

更多 ONNX 用法请参考 [ONNX 生产化完整指南](ONNX.md)。
