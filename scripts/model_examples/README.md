# Model Examples

本目录提供按模型提供商分类的最小可运行示例，帮助你快速验证各后端连接。

## 运行方式

每个子目录都是独立的 Go 程序，进入目录后运行：

```bash
cd scripts/model_examples/openai_chat_call
export OPENAI_API_KEY=sk-...
go run .
```

## 示例列表

| 示例 | 提供商 | 所需环境变量 |
|------|--------|--------------|
| `openai_chat_call` | OpenAI Chat Completions | `OPENAI_API_KEY` |
| `openai_chat_multiagent` | OpenAI Chat + MsgHub 多 Agent | `OPENAI_API_KEY` |
| `openai_chat_stream` | OpenAI Chat + V2 事件流 | `OPENAI_API_KEY` |
| `openai_chat_multimodal` | OpenAI Chat + 图片输入 | `OPENAI_API_KEY` |
| `openai_response_call` | OpenAI Responses API (o3/o4-mini) | `OPENAI_API_KEY` |
| `anthropic_call` | Anthropic Claude | `ANTHROPIC_API_KEY` |
| `gemini_call` | Google Gemini | `GEMINI_API_KEY` |
| `dashscope_call` | 阿里云 DashScope / 通义千问 | `DASHSCOPE_API_KEY` |
| `deepseek_call` | DeepSeek | `DEEPSEEK_API_KEY` |
| `moonshot_call` | Moonshot / Kimi | `MOONSHOT_API_KEY` |
| `xai_call` | xAI / Grok | `XAI_API_KEY` |
| `ollama_call` | 本地 Ollama | 无需 key，需本地启动 Ollama |
| `vllm_call` | 私有化 vLLM | 无需 key，需本地启动 vLLM |

## 批量构建验证

```bash
go build ./scripts/model_examples/...
```

## 后续扩展

计划补充：

- `*_multiagent.go`：多 Agent 对话示例
- `*_multimodal.go`：图片/音频输入示例
- `*_stream.go`：事件流式消费示例
