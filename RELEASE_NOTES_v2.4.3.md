# AgentScope.Go v2.4.3 Release Notes

> 🚀 **AgentScope.Go v2.4.3** —— 模型层健壮性：Anthropic 流式工具调用 + 流中途错误传播。

---

## 核心亮点

### 1. Anthropic 流式工具调用（修复真实 bug）

此前 `model/anthropic` 的流式模式会**静默丢弃工具调用**（仅非流式可用），导致 ReAct Agent 在流式下无法调度工具。本次修复：

- **SSE tool_use 累积**：`content_block_start`（tool_use）+ `content_block_delta`（`input_json_delta`）按 content_block index 累积，流结束时按序交付 `ToolUseBlock`（含 RawInput，支持分片 JSON 与转义）
- **`message_stop` 终止信号**：Anthropic 标准流终止事件（而非 OpenAI 风格 `[DONE]`），`[DONE]` 分支保留作代理/网关兼容
- **`WithMaxTokens` 覆盖**：`ChatOptions.MaxTokens > 0` 覆盖 Builder 默认值，与 OpenAI/Gemini/openai_response 后端对齐（此前被忽略）

### 2. 流中途错误传播（`StreamChunk.Error` 公共字段扩展）

新增 `model.StreamChunk.Error` 字段（向后兼容，零值 nil），流中途错误不再静默产出空响应：

- **后端填充**：Anthropic SSE `{"type":"error"}` 事件、OpenAI `ChatCompletionStream.Recv()` 中途错误
- **消费端**：react V1 主循环（fire ErrorEvent + 返回错误）、middleware 包装流、V2 `runModelStream`（发 `event.NewError` + `ModelCallEnd`）、`StructuredRunner.RunStream`（经 `StreamResult.Err`，不再误报 "JSON parse failed"）

### 3. 测试覆盖

- `model/anthropic`：错误事件透传（含无 message 字段的默认文本）/ message_stop 终止 / 多 tool_use 乱序排序 / text+tool 混合 / 空 `{}` input / 分片+转义 JSON / `MaxTokens` 覆盖 / **真实 API 集成测试**（`ANTHROPIC_API_KEY` 驱动，未设置自动 skip）
- `agent/react`：V1 `runModel`、middleware 包装闭包、V2 `ReplyStream` 三条流中途错误传播路径
- `output`：流中途模型错误经 `StreamResult.Err` 传播

### 4. 文档

- `CHANGELOG.md` 新增 v2.4.3 章节
- `docs/api-reference.md`（含 docs-site 镜像）补充 `StreamChunk` 字段语义（Delta/IsThinking/Content/Done/Usage/Error）

---

## 完整改动

- 13 个文件，+731 / -19 行
- 提交：`c17f344`（feat(anthropic): stream tool_use accumulation + mid-stream error propagation）
