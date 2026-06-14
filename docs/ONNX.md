# ONNX 生产化完整指南

AgentScope.Go 提供完整的 ONNX 推理基础设施，支持图像/音频预处理、CLIP/Whisper 嵌入、模型管理和跨模态检索。采用 HTTP 代理方案，无需在 Go 进程中加载 ONNX Runtime，实现推理与业务解耦。

---

## 1. 架构概览

```
┌─────────────────┐     HTTP/JSON      ┌──────────────────┐
│   Go Agent      │ ◄────────────────► │  ONNX Service    │
│  (本指南)       │   embedding/onnx   │  (Python/FastAPI)│
└─────────────────┘                    └──────────────────┘
         │
    ┌────┴────┐
    ▼         ▼
ImagePipe  AudioPipe
```

- **Go 端**：负责预处理、嵌入器调用、模型管理、跨模态对齐
- **ONNX Service**：负责实际推理（可用 FastAPI + ONNX Runtime 搭建）

---

## 2. 图像预处理管道

### 2.1 基本用法

```go
import "github.com/linkerlin/agentscope.go/embedding/onnx"

// 创建 CLIP 图像预处理器（默认 224x224, NCHW）
pre := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())

// 从 io.Reader 预处理
tensor, err := pre.Preprocess(imageFile)
// tensor 形状: [1, 3, 224, 224] = 150528 个 float32

// 从字节数组预处理
tensor, err := pre.PreprocessBytes(imageBytes)
```

### 2.2 配置项

| 字段 | 默认值 | 说明 |
|------|--------|------|
| Width | 224 | 目标宽度 |
| Height | 224 | 目标高度 |
| Mean | [0.481, 0.458, 0.408] | ImageNet RGB 均值 |
| Std | [0.269, 0.261, 0.276] | ImageNet RGB 标准差 |
| Normalize | true | 是否归一化 |
| ToTensor | true | 是否转 NCHW |
| Interpolate | bilinear | 插值方法 |

### 2.3 完整示例

```go
package main

import (
    "bytes"
    "fmt"
    "image"
    "image/color"

    "github.com/linkerlin/agentscope.go/embedding/onnx"
)

func main() {
    // 生成 64x64 合成图像
    img := image.NewRGBA(image.Rect(0, 0, 64, 64))
    for y := 0; y < 64; y++ {
        for x := 0; x < 64; x++ {
            img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
        }
    }

    // 预处理为 CLIP 输入格式
    pre := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())
    tensor, err := pre.PreprocessImage(img)
    if err != nil {
        panic(err)
    }
    fmt.Printf("image tensor len=%d (expected %d)\n", len(tensor), 1*3*224*224)
}
```

> 完整示例见 [`examples/onnx/main.go`](../examples/onnx/main.go)

---

## 3. 音频预处理管道

### 3.1 基本用法

```go
// 创建 Whisper 音频预处理器（默认 16kHz, 80 mels, 3000 frames）
audioPre := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())

// 预处理 PCM 样本
melTensor, err := audioPre.Preprocess(samples, 16000)
// melTensor 形状: [1, 80, 3000] = 240000 个 float32
```

### 3.2 配置项

| 字段 | 默认值 | 说明 |
|------|--------|------|
| SampleRate | 16000 | 目标采样率 |
| NumSamples | 480000 | 30s 最大样本数 |
| NFFT | 400 | FFT 窗口大小 |
| HopLength | 160 | 帧移 |
| NMels | 80 | Mel 滤波器数量 |
| NFrames | 3000 | 目标帧数 |
| Normalize | true | 是否归一化 |
| PadToMaxLength | true | 是否填充到最大长度 |

### 3.3 完整示例

```go
// 生成 1 秒合成 PCM @ 16kHz
samples := make([]float32, 16000)
for i := range samples {
    samples[i] = float32(i%100) / 100.0
}

audioPre := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())
melTensor, err := audioPre.Preprocess(samples, 16000)
if err != nil {
    panic(err)
}
fmt.Printf("mel tensor len=%d (expected %d)\n", len(melTensor), 1*80*3000)
```

---

## 4. CLIP 图像嵌入器

CLIPImageEmbedder 通过 HTTP 调用 ONNX 服务获取图像嵌入向量。

```go
// 创建嵌入器
embedder := onnx.NewCLIPImageEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
// BaseURL 默认: http://localhost:8000

// 方法 1: 直接嵌入预处理后的图像数据
vec, err := embedder.EmbedImage(preprocessedTensor)

// 方法 2: 从原始图像字节完整嵌入（自动预处理）
vec, err := embedder.EmbedImageFromBytes(imageBytes)

// 方法 3: 批量嵌入
vecs, err := embedder.BatchEmbedImages([][]float32{tensor1, tensor2})

// 健康检查
err := embedder.HealthCheck()

// 获取模型信息
info, err := embedder.GetModelInfo()
```

### CLIP 文本嵌入器（跨模态对齐）

```go
textEmbedder := onnx.NewCLIPTextEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
textVec, err := textEmbedder.EmbedText("a photo of a cat")
```

---

## 5. Whisper 音频嵌入器

WhisperAudioEmbedder 通过 HTTP 调用 ONNX 服务获取音频嵌入向量或转录文本。

```go
// 创建嵌入器
embedder := onnx.NewWhisperAudioEmbedder(onnx.DefaultWhisperAudioEmbedderConfig())

// 方法 1: 嵌入预处理后的 Mel 频谱图
vec, err := embedder.EmbedAudio(melTensor)

// 方法 2: 从原始 PCM 完整嵌入（自动预处理）
vec, err := embedder.EmbedAudioFromPCM(samples, 16000)

// 方法 3: 转录音频为文本
text, err := embedder.TranscribeAudio(melTensor)

// 批量嵌入
vecs, err := embedder.BatchEmbedAudio([][]float32{mel1, mel2})
```

---

## 6. 模型管理器

ModelManager 负责 ONNX 模型的下载、缓存和版本管理。

```go
// 创建管理器
mgr, err := onnx.NewModelManager(onnx.DefaultModelManagerConfig())
// 默认缓存目录: ~/.agentscope/onnx_models
// 默认最大缓存: 10GB

// 注册模型
mgr.RegisterModel(onnx.ModelInfo{
    Name:        "clip-vit-base-patch32",
    Version:     "1.0",
    URL:         "https://huggingface.co/.../model.onnx",
    Size:        300 * 1024 * 1024,
    Description: "CLIP ViT-B/32",
})

// 下载模型（自动下载到缓存目录）
err := mgr.DownloadModel("clip-vit-base-patch32", "1.0")

// 获取模型本地路径（自动下载）
path, err := mgr.GetModelPath("clip-vit-base-patch32", "1.0")

// 列出所有模型
models := mgr.ListModels()

// 删除模型
err := mgr.RemoveModel("clip-vit-base-patch32", "1.0")

// 清理缓存（LRU 策略）
err := mgr.CleanupCache()
```

### 预定义模型

```go
models := onnx.PredefinedModels()
// clip-vit-base-patch32 (~300MB)
// clip-vit-base-patch16 (~600MB)
// whisper-base (~150MB)
// whisper-small (~500MB)
```

---

## 7. 跨模态相似度

计算图像-文本、音频-文本的跨模态余弦相似度。

```go
// 图像-文本相似度
sim, err := onnx.CrossModalSimilarity(imageVec, textVec)
// 返回: [-1, 1] 的 float32

// 音频-文本对齐器
alignment := onnx.NewAudioTextAlignment(audioEmbedder, textEmbedder)
score, err := alignment.Align(melTensor, "这段音频说的是什么")
```

### 跨模态检索示例

```go
// 文本搜图像
imageResults := searcher.SearchByText(ctx, "sunset over ocean", 5)

// 文本搜音频
audioResults := searcher.SearchByText(ctx, "someone saying hello", 5)
```

> 完整跨模态检索示例见 [`examples/cross_modal/main.go`](../examples/cross_modal/main.go)

---

## 8. HTTP 代理方案架构

### 8.1 服务端（Python FastAPI 示例）

```python
from fastapi import FastAPI
from fastapi.responses import JSONResponse
import onnxruntime as ort
import numpy as np

app = FastAPI()
session = ort.InferenceSession("clip-vit-base-patch32.onnx")

@app.post("/embed/image")
async def embed_image(req: dict):
    image_data = np.array(req["image_data"], dtype=np.float32)
    image_data = image_data.reshape(1, 3, 224, 224)
    outputs = session.run(None, {"input": image_data})
    return {
        "embedding": outputs[0].tolist(),
        "model": "clip-vit-base-patch32",
        "dim": outputs[0].shape[-1]
    }

@app.get("/health")
async def health():
    return {"status": "ok"}
```

### 8.2 客户端（Go AgentScope）

```go
// 配置 ONNX 服务端点
config := onnx.CLIPImageEmbedderConfig{
    BaseURL: "http://onnx-service:8000",
    Timeout: 30 * time.Second,
}
embedder := onnx.NewCLIPImageEmbedder(config)

// 在 Agent 中使用
agent, _ := react.Builder().
    Name("VisionAgent").
    Model(chatModel).
    ImageEmbedder(embedder). // 假设 Builder 支持
    Build()
```

### 8.3 部署拓扑

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Go Gateway │────►│  ONNX Svc   │────►│  GPU Node   │
│  (多实例)   │     │  (多实例)   │     │  (模型加载) │
└─────────────┘     └─────────────┘     └─────────────┘
       │
       ▼
┌─────────────┐
│  Model Mgr  │
│  (缓存/下载)│
└─────────────┘
```

---

## 9. 完整流水线示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/linkerlin/agentscope.go/embedding/onnx"
    "github.com/linkerlin/agentscope.go/memory"
)

func main() {
    ctx := context.Background()

    // 1. 图像嵌入
    imgEmbedder := onnx.NewCLIPImageEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
    imgVec, _ := imgEmbedder.EmbedImageFromBytes(imageBytes)

    // 2. 文本嵌入
    textEmbedder := onnx.NewCLIPTextEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
    textVec, _ := textEmbedder.EmbedText("a photo of a cat")

    // 3. 计算相似度
    sim, _ := onnx.CrossModalSimilarity(imgVec, textVec)
    fmt.Printf("similarity: %.3f\n", sim)

    // 4. 存入向量记忆
    store := memory.NewLocalVectorStore(textEmbedder)
    node := memory.NewMemoryNode(memory.MemoryTypeHistory, "demo", "cat.jpg")
    node.Vector = imgVec
    store.Insert(ctx, []*memory.MemoryNode{node})
}
```

---

## 10. 相关文件

- `embedding/onnx/image_preprocess.go` — 图像预处理管道
- `embedding/onnx/audio_preprocess.go` — 音频预处理管道
- `embedding/onnx/clip_image.go` — CLIP 图像/文本嵌入器
- `embedding/onnx/whisper_audio.go` — Whisper 音频嵌入器
- `embedding/onnx/model_manager.go` — 模型管理器
- `examples/onnx/main.go` — ONNX 完整示例
- `examples/cross_modal/main.go` — 跨模态检索示例
