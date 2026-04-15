# Python 版 AgentScope 2.0 核心架构研究报告

> 研究范围：`C:\GitHub\agentscope`（Python 版）  
> 重点 commits：`b158fc2e`、`f894a1c6`、`36a673c4`、`7646541b`  
> 报告日期：2026-04-15

---

## 1. 执行摘要

Python 版 AgentScope 在 2026 年 3 月中旬至 4 月中旬期间完成了一次**向 2.0 架构的彻底重构**。这次重构的核心目标是：

1. **统一消息原语**：将原先分散的 `ImageBlock` / `AudioBlock` / `VideoBlock` 等多媒体块统一为 `DataBlock`，并引入 `ToolCallBlock`、`ToolResultBlock`、`ThinkingBlock`、`HintBlock` 等语义化块，形成一套完整、可验证的 `Msg + Block` 体系。
2. **事件驱动化**：把 Agent 的推理-行动（Reasoning-Acting）循环拆分为细粒度的事件流（`AgentEvent`），支持流式消费、用户确认、外部执行等中断-恢复模式。
3. **职责边界清晰化**：`Formatter` 负责把 `Msg` 转成模型 API 需要的字典格式；`Model` 负责重试、降级和调用；`Toolkit` 负责工具注册、分组、MCP 集成、异步执行；`Agent` 负责状态管理与事件编排。
4. **瘦身核心**：`evaluate`、`rag`、`tts`、`realtime` 等非核心模块被临时移出主干，减少维护面，聚焦 2.0 最小可用核心。

**对 Go 版（agentscope.go）的最关键启示**：Go 版目前的消息块、事件体系、Agent 接口、Formatter 抽象均与 Python 2.0 存在明显代差。若希望与 Python 版在协议、存储、多 Agent 协同上保持互操作，需要在新的事件模型、统一的 `DataBlock`、细粒度的 `AgentEvent`、以及 `Toolkit` 的 MCP / 异步执行 / 分组动态激活等能力上进行系统性对齐。

---

## 2. 核心架构分层图（文字版）

```
┌─────────────────────────────────────────────────────────────┐
│  Application / Orchestrator / A2A / Studio UI               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Agent (统一 Agent 类)                                       │
│  - reply() / reply_stream()                                  │
│  - _reply(): 事件驱动的 Reasoning-Acting 循环               │
│  - AgentState (context, reply_id, cur_iter, cur_summary)   │
│  - load_state() / save_state()                               │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌─────────────────┐    ┌──────────────┐
│   Event       │    │    Model        │    │   Toolkit    │
│  (_event.py)  │◄───│  (_model_base)  │    │ (_toolkit.py)│
│  AgentEvent   │    │  - __call__     │    │ - register   │
│  细粒度事件流  │    │  - retry/fallback│   │ - groups     │
│  (start/delta/│    │  - formatter    │    │ - MCP        │
│   end/pause)  │    │                 │    │ - async exec │
└───────────────┘    └─────────────────┘    └──────────────┘
        ▲                     │                     │
        │                     ▼                     ▼
        │           ┌─────────────────┐      ┌──────────────┐
        │           │   Formatter     │      │   Tool       │
        │           │(_formatter_base)│      │ (_response)  │
        │           │ - format()      │      │ - ToolResponse│
        │           │ - multimodal    │      │ - streaming  │
        │           │   fallback      │      └──────────────┘
        │           └─────────────────┘
        │                     │
        ▼                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Message (_base.py + _block.py)                             │
│  Msg: name, role, content(str|list[ContentBlock]), metadata │
│  Blocks: Text | Thinking | Hint | Data | ToolCall | ToolResult│
└─────────────────────────────────────────────────────────────┘
```

---

## 3. 消息系统设计详解

### 3.1 Msg 基类

`Msg` 是 Pydantic `BaseModel`，核心字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | `str` | 发送者名称 |
| `content` | `str \| list[ContentBlock]` | 消息内容，既支持纯文本字符串（兼容旧用法），也支持结构化块列表 |
| `role` | `Literal["user","assistant","system"]` | 发送者角色 |
| `id` | `str` | 消息唯一 ID（shortuuid） |
| `metadata` | `dict` | 扩展元数据 |
| `created_at` | `str` | ISO 8601 时间戳 |

**关键设计点**：
- `content` 保留了 `str` 的向后兼容能力，但在内部处理时会被 Formatter 隐式转为 `TextBlock`。
- `model_validator(mode="after")` 对 `role` 与 `content` 的类型组合做了强制校验：
  - `user`：只允许 `text` 和 `data` 块；
  - `system`：只允许 `text` 块；
  - `assistant`：不限制（可包含 `tool_call`、`thinking` 等）。

### 3.2 Block 体系

在 `_block.py` 中定义了 6 种核心块：

| 块类型 | 关键字段 | 语义 |
|--------|----------|------|
| `TextBlock` | `text: str` | 纯文本 |
| `ThinkingBlock` | `thinking: str` | 模型推理链（Chain-of-Thought） |
| `HintBlock` | `hint: str` | 给 LLM 的系统级提示/指令，在传入 API 时会被 Formatter 转成一条 user message |
| `DataBlock` | `source: Base64Source \| URLSource`, `name: str \| None` | **统一的多媒体块**，可承载 image/audio/video 等任意二进制数据 |
| `ToolCallBlock` | `id, name, input: str, await_user_confirmation: bool` | 工具调用请求；`input` 是原始 JSON 字符串（流式场景下会增量累积） |
| `ToolResultBlock` | `id, name, output: str \| list[TextBlock\|DataBlock], state` | 工具执行结果；`state` 枚举为 `success/error/interrupted/running` |

**架构意图**：
- **去类型爆炸**：旧版有独立的 `ImageBlock`、`AudioBlock`、`VideoBlock`，新版把它们全部收敛到 `DataBlock`，通过 `media_type` 区分 MIME 类型，避免每新增一种媒体就要新增一个类。
- **流式友好**：`ToolCallBlock.input` 使用 `str` 而不是 `dict`，是因为流式模型返回的 tool call 参数往往是 JSON 片段，需要拼接后再解析。
- **状态显式化**：`ToolResultBlock.state` 让工具执行的生命周期在消息层就可见，方便事件系统做 `TOOL_RESULT_END` 等状态转换。

### 3.3 工厂函数

提供了三个便捷构造函数：
- `UserMsg(name, content, ...)`
- `AssistantMsg(name, content, ...)`
- `SystemMsg(name, content, ...)`

这些工厂函数在内部自动注入当前时间戳和空 metadata，降低用户侧的心智负担。

---

## 4. 事件系统详解

### 4.1 事件模型

`src/agentscope/event/_event.py` 定义了基于 Pydantic 的强类型事件体系。所有事件继承 `EventBase`（含 `id` 和 `created_at`）。

事件分为以下几大类：

**1. Run 生命周期**
- `RUN_STARTED` / `RUN_FINISHED`
- `EXCEED_MAX_ITERS`

**2. Model 调用生命周期**
- `MODEL_CALL_STARTED` / `MODEL_CALL_ENDED`（携带 input_tokens / output_tokens）

**3. 内容块流式事件（Block 级别）**
- `TEXT_BLOCK_START` / `TEXT_BLOCK_DELTA` / `TEXT_BLOCK_END`
- `THINKING_BLOCK_START` / `THINKING_BLOCK_DELTA` / `THINKING_BLOCK_END`
- `BINARY_BLOCK_START` / `BINARY_BLOCK_DELTA` / `BINARY_BLOCK_END`

**4. 工具调用与结果流式事件**
- `TOOL_CALL_START` / `TOOL_CALL_DELTA`（增量 JSON 参数） / `TOOL_CALL_END`
- `TOOL_RESULT_START` / `TOOL_RESULT_TEXT_DELTA` / `TOOL_RESULT_BINARY_DELTA` / `TOOL_RESULT_END`（携带最终 `state`）

**5. 交互式中断-恢复事件**
- `REQUIRE_USER_CONFIRM` / `REQUIRE_EXTERNAL_EXECUTION`
- `USER_CONFIRM_RESULT` / `EXTERNAL_EXECUTION_RESULT`

### 4.2 事件流与 Agent 的交互

`Agent.reply_stream()` 的返回值类型是 `AsyncGenerator[AgentEvent, None]`。核心循环 `_reply()` 的工作方式：

1. **初始化**：若不在等待状态，则生成 `RunStartedEvent`，重置 `reply_id` 和 `cur_iter`。
2. **追加观察**：把传入的 `msgs` 追加到 `context`。
3. **Reasoning-Acting 循环**（最多 `max_iters` 轮）：
   - 若上下文末尾没有待执行的 `ToolCallBlock`，则调用 `_reasoning()` 发起模型调用；
   - `_reasoning()` 内部把模型的 `ChatResponse` 流转换为上述细粒度事件，并在流结束后把完整块保存到 `context`；
   - 若产生新的 `ToolCallBlock`，则进入 `_acting()`；
   - `_acting()` 目前尚未接入 Toolkit（代码中为 `raise NotImplementedError`），但设计上会产出 `ToolResult*` 事件并把结果写回 `context`。
4. **中断-恢复**：若 Toolkit 判定某工具需要用户确认或外部执行，Agent 会 yield `RequireUserConfirmEvent` 或 `RequireExternalExecutionEvent`，并 yield 一条等待中的 `Msg` 后 `return`。外部调用方在拿到确认结果或外部执行结果后，需要再次调用 `reply_stream(event=...)` 把对应事件传回 Agent，Agent 会从中断点继续。

### 4.3 架构意图

- **一切皆事件**：Agent 内部不再直接打印或返回裸字符串，而是通过结构化事件与外部世界通信。这使得 UI、日志、追踪、A2A 等上层系统都能以统一方式消费 Agent 行为。
- **流式粒度下沉到块**：不仅模型输出可以流式消费，工具结果的二进制数据、文本数据也可以流式消费，避免大文件或长文本一次性加载进内存。
- **可中断的协程**：通过 `awaiting_type` + `expected_event_type` 机制，Agent 的 `_reply()` 可以在任意轮次暂停，等待外部输入后再恢复，天然支持人机协同和审批流。

---

## 5. Agent 生命周期与职责

### 5.1 Agent 基类（`_agent.py`）

Python 2.0 的 `Agent` 是一个**统一的、非抽象的类**，意图是取代旧版繁杂的 `AgentBase` / `ReActAgentBase` / `UserAgent` 等多层继承。

**核心职责**：
- **状态容器**：`AgentState`（`context`, `reply_id`, `cur_iter`, `cur_summary`）。
- **上下文管理**：`_save_to_context()` 会把 Assistant 产生的块追加到 `context` 最后一条 assistant message 中，或新建一条 message。
- **工具调用跟踪**：`_get_pending_tool_calls()` 自动从最后一条 assistant message 中找出尚未有 `ToolResultBlock` 对应的 `ToolCallBlock`。
- **记忆压缩**：`_compress_memory_if_needed()` 和 `_split_context_for_compression()` 目前为 TODO，但已预留了基于 `cur_summary` 的压缩位点。

**入口方法**：
- `reply(msgs, event=None) -> Msg`：阻塞式调用，消费所有事件，返回最终聚合的 `Msg`。
- `reply_stream(msgs, event=None) -> AsyncGenerator[AgentEvent, None]`：流式调用，产出事件。
- `_observe(msgs)`：仅把外部消息追加到 `context`，不触发推理。

### 5.2 与 Model / Formatter / Toolkit 的协作

| 组件 | Agent 如何使用 |
|------|----------------|
| `Model` | 在 `_reasoning()` 中调用 `self.model(messages=..., tool_choice=...)`；模型返回 `ChatResponse`（或异步生成器）。 |
| `Formatter` | 被挂载在 `ChatModelBase` 上，由 `Model.__call__()` 在调用 `_call_api()` 之前自动执行 `self.formatter.format(messages)`。Agent 不直接感知 Formatter。 |
| `Toolkit` | 在 `_acting()` 中调用 `toolkit.call_tool_function(tool_call)`；Toolkit 返回 `ToolResponse` 异步生成器，Agent 再把它转成事件。 |

---

## 6. Formatter / Model / Tool 之间的边界

### 6.1 Formatter

`FormatterBase` 的核心契约：
- `async def format(...) -> list[dict[str, Any]]`：把 `list[Msg]` 转成模型 API 所需的字典列表。
- `convert_tool_result_to_string()`：处理**不支持多模态 tool result** 的模型 API（如早期 OpenAI）。其策略是：
  - 若 `DataBlock` 的 MIME 类型在 `supported_input_media_types` 内，则将其提升为独立的用户消息块（并附带系统提示的 identifier）；
  - 若为 URL 则直接在文本中引用；
  - 若为 base64 则保存为本地临时文件并在文本中给出路径。
- `_group_messages()`：把消息序列分成 `agent_message` 组和 `tool_sequence` 组，方便不同模型 API（如 Anthropic 要求 tool call 和 tool result 必须成对出现）进行格式编排。

**边界**：Formatter 只负责**结构转换和降级兼容**，不发起网络请求，也不持有业务状态。

### 6.2 Model

`ChatModelBase` 的核心契约：
- `__call__()`：编排层。先执行 Formatter（如果有），然后进入 retry 循环，再尝试 fallback model。
- `_call_api()`：子类必须实现，真正发起模型请求。
- `_validate_tool_choice()`：校验 `tool_choice` 参数合法性。

**边界**：Model 只负责**网络请求、流式解析、重试降级**。它接收 `list[Msg]`，但内部会把它变成 API 字典（通过 Formatter），最终返回 `ChatResponse`（或其异步生成器）。

### 6.3 Toolkit

`Toolkit` 的核心契约：
- **注册**：`register_tool_function()` 自动从 docstring 提取 JSON Schema，支持 `preset_kwargs`（对 Agent 隐藏）、`postprocess_func`、命名冲突策略（raise/override/skip/rename）、异步执行标记。
- **分组管理**：`create_tool_group()` / `update_tool_groups()`；`basic` 组始终激活，其余组需显式激活。`get_json_schemas()` 只返回激活组的 schema。
- **MCP 集成**：`register_mcp_client()` 可把 MCP server 的工具一键注册到 Toolkit。
- **执行**：`call_tool_function(tool_call: ToolCallBlock)` 返回 `AsyncGenerator[ToolResponse, None]`，统一了同步/异步/流式/非流式工具的实现差异。
- **异步执行**：若工具标记了 `async_execution=True`，Toolkit 会创建后台 `asyncio.Task`，并立即返回一个带 `task_id` 的提示消息，Agent 后续可通过 task_id 查询或取消。
- **中间件**：`_apply_middlewares` 装饰器支持在运行时动态构建中间件链，用于拦截、修改或重试工具调用。
- **元工具（Meta Tool）**：`reset_equipped_tools` 是一个内置的动态 schema 工具，Agent 可调用来激活/关闭工具组。

**边界**：Toolkit 只负责**工具生命周期管理与执行**，不直接操作 Agent 的上下文。执行结果以 `ToolResponse` 形式返回，由 Agent 自行决定如何追加到 `context`。

---

## 7. 与旧版相比的架构变化

| 维度 | 旧版（1.x） | 新版（2.0，2026-04） |
|------|-------------|----------------------|
| **消息块** | `ImageBlock` / `AudioBlock` / `VideoBlock` / `ToolUseBlock` 等独立类 | 统一为 `DataBlock`（image/audio/video 仅通过 `media_type` 区分）；`ToolUseBlock` 重命名为 `ToolCallBlock`；新增 `ThinkingBlock`、`HintBlock` |
| **Msg 校验** | 较弱，无 role-content 联动校验 | `model_validator` 强制校验：user 只能有 text/data，system 只能有 text |
| **Agent 体系** | 多层继承：`AgentBase` -> `ReActAgentBase` -> `ReActAgent` | 统一为单个 `Agent` 类，内部通过 `_reasoning()` / `_acting()` / `_reply()` 组织循环 |
| **流式模型** | 模型层返回 chunk，Agent 层直接处理 | 引入 **Formatter + Event** 双层转换：Model -> ChatResponse -> AgentEvent -> 外部消费 |
| **事件系统** | 无独立事件模块，依赖 `Msg` 和打印 | 新增 `event/_event.py`，定义 20+ 种细粒度强类型事件，作为 Agent 与外部交互的第一公民 |
| **工具执行** | 直接函数调用，返回字符串或 dict | `Toolkit` 统一返回 `AsyncGenerator[ToolResponse]`，支持流式、异步执行、中间件、MCP |
| **记忆/上下文** | 简单的列表追加 | 引入 `AgentState`、`cur_summary`、预留 `_compress_memory_if_needed()` 压缩位点 |
| **模块范围** | 大而全（rag、tts、realtime、evaluate 都在核心包内） | **瘦身核心**：`36a673c4` 将 evaluate/rag/tts/realtime 等模块及大量 example 临时移出主干，聚焦 2.0 最小核心 |
| **A2A/网络发现** | 有 `_file_resolver`、`_nacos_resolver` 等复杂发现机制 | 在 `b158fc2e` 中被移除，A2A 能力预计会以更轻量的方式重建 |

---

## 8. 对 Go 版（agentscope.go）的启示

以下按模块列出 Go 版需要**借鉴、对齐或重构**的具体点。

### 8.1 消息层（message）

**现状差距**：Go 版目前仍有独立的 `ImageBlock`、`AudioBlock`、`VideoBlock`，且 `ToolUseBlock` 的 `Input` 是 `map[string]any`，`ToolResultBlock` 的字段也与 Python 2.0 不一致。

**建议行动**：
1. **统一多媒体块**：删除 `ImageBlock` / `AudioBlock` / `VideoBlock`，新增 `DataBlock`：
   ```go
   type DataBlock struct {
       Source   Source      // Base64Source | URLSource
       MediaType string
       Name      string
   }
   type Base64Source struct { Data string; MediaType string }
   type URLSource    struct { URL    string; MediaType string }
   ```
2. **重命名并对齐 ToolCallBlock**：
   - 将 `ToolUseBlock` 改名为 `ToolCallBlock`。
   - `Input` 字段类型从 `map[string]any` 改为 `string`（承载原始 JSON），以支持流式增量累积。
   - 新增 `AwaitUserConfirmation bool`。
3. **对齐 ToolResultBlock**：
   ```go
   type ToolResultBlock struct {
       ID     string
       Name   string
       Output string | []ContentBlock   // 可用 interface{} 或自定义 union
       State  string // "success" | "error" | "interrupted" | "running"
   }
   ```
4. **新增 HintBlock**：用于 ReAct 循环中给模型插入系统提示但又不污染 system prompt。
5. **角色校验**：在 `UserMsg` / `SystemMsg` / `AssistantMsg` 的构造函数或 Build 方法中加入校验逻辑（如 user 只允许 `*TextBlock` 和 `*DataBlock`）。
6. **Msg 字段对齐**：Python 版 `Msg.content` 支持 `str | list[ContentBlock]`，Go 版当前只支持 `[]ContentBlock`。建议 Go 版也保留 `string` 兼容路径（例如增加 `ContentString string` + `ContentBlocks []ContentBlock` 的双字段设计，或让 `Content` 为 `interface{}`）。

### 8.2 事件层（hook / event）

**现状差距**：Go 版 `hook/events.go` 定义的事件较粗粒度，只有 `PreReasoning` / `PostReasoning` / `ReasoningChunk` / `PreActing` / `PostActing` / `ActingChunk` / `Error`，缺少块级别的 start/end、binary 增量、run 生命周期、用户确认/外部执行等事件。

**建议行动**：
1. **扩展事件类型枚举**：在 `hook/events.go` 中新增与 Python 2.0 对应的事件类型：
   - `RunStarted` / `RunFinished` / `ExceedMaxIters`
   - `ModelCallStarted` / `ModelCallEnded`
   - `TextBlockStart` / `TextBlockDelta` / `TextBlockEnd`
   - `ThinkingBlockStart` / `ThinkingBlockDelta` / `ThinkingBlockEnd`
   - `BinaryBlockStart` / `BinaryBlockDelta` / `BinaryBlockEnd`
   - `ToolCallStart` / `ToolCallDelta` / `ToolCallEnd`
   - `ToolResultStart` / `ToolResultTextDelta` / `ToolResultBinaryDelta` / `ToolResultEnd`
   - `RequireUserConfirm` / `RequireExternalExecution`
   - `UserConfirmResult` / `ExternalExecutionResult`
2. **事件字段对齐**：每个事件应携带 `reply_id`（对应 Python 的 `reply_id`），块事件携带 `block_id` 或 `tool_call_id`。
3. **Agent 流式方法改造**：Go 版目前 `CallStream` 返回 `<-chan *message.Msg`，应改为返回 `<-chan Event`（或一个联合类型 `StreamItem`），其中既包含事件也包含最终的 `Msg`（或把最终 `Msg` 作为 `RunFinishedEvent` 的 payload）。
4. **中断-恢复协议**：Go 版 ReActAgent 目前不支持在工具调用前挂起等待外部输入。建议借鉴 Python 的 `awaiting_type` 机制，在 `reply_stream` 中支持挂起并等待 `UserConfirmResultEvent` / `ExternalExecutionResultEvent` 后恢复。

### 8.3 Agent 层

**现状差距**：Go 版 `Agent` 是一个极简接口（`Call` / `CallStream`），没有统一的状态管理、没有 `reply_id`、没有 `AgentState`，ReAct 逻辑分散在 `agent/react/react_agent.go` 中。

**建议行动**：
1. **引入统一 Agent 结构体**：仿照 Python 2.0 设计一个 `BaseAgent`（或直接把现有 `react.Agent` 重构为统一 Agent），包含：
   - `Name string`
   - `SysPrompt string`
   - `Model model.ChatModel`
   - `MaxIters int`
   - `Context []*message.Msg`
   - `ReplyID string`
   - `CurIter int`
   - `CurSummary string`
2. **状态持久化接口**：定义 `StateLoader` / `StateSaver` 接口，在 `reply_stream` 前后自动调用，与 Python 的 `load_state()` / `save_state()` 对齐。
3. **事件转换函数**：在 Agent 内部增加 `_convertChatResponseToEvents` 和 `_convertToolResponseToEvents`，把 Model 和 Toolkit 的输出翻译成 `hook.Event` 流。
4. **上下文追加策略**：复现 Python 的 `_save_to_context` 逻辑：如果最后一条 message 是同一 assistant 的，就把新块 extend 进去；否则新建 message。
5. **挂起-恢复状态机**：在 `_reply` 循环中引入 `expectedEventType` 判断，支持从外部事件恢复执行。

### 8.4 Formatter 层

**现状差距**：Go 版目前没有独立的 Formatter 抽象，模型实现各自处理 `message.Msg` 到 API 字典的转换。

**建议行动**：
1. **新建 `formatter` 包**：定义 `Formatter` 接口：
   ```go
   type Formatter interface {
       Format(msgs []*message.Msg) ([]map[string]any, error)
       SupportedMediaTypes() []string
       ConvertToolResult(output interface{}) (string, []message.ContentBlock, error)
   }
   ```
2. **分组逻辑**：实现 `_groupMessages`，把消息列表按 `tool_sequence` / `agent_message` 分组，方便不同模型（OpenAI、Anthropic、Gemini）的格式化器实现。
3. **多模态降级**：在 Formatter 中统一处理 tool result 里的 `DataBlock`：若模型不支持该 media type，则保存 base64 为本地临时文件并在文本中引用路径。

### 8.5 Model 层

**现状差距**：Go 版 `ChatModel` 接口只有 `Chat` / `ChatStream` / `ModelName`，没有重试、降级、Formatter 挂载点。

**建议行动**：
1. **在 Model 实现中内置重试与降级**：可参考 Python `ChatModelBase.__call__()` 的实现，在模型 wrapper 层（而非每个 provider 内部）统一实现 retry + fallback。
2. **挂载 Formatter**：在 `ChatModel` 的构造函数或配置中接受 `Formatter`，并在调用 provider 前自动执行 `Format()`。
3. **流式返回统一**：`ChatStream` 返回的 `StreamChunk` 目前只有 `Delta string` 和 `Content []message.ContentBlock`，建议增加：
   - `IsLast bool`
   - `Usage *ChatUsage`
   - 若模型返回 thinking / tool_call，也应通过 `Content []message.ContentBlock` 传递（当前已支持，但需确保各 provider 正确填充）。

### 8.6 Toolkit 层

**现状差距**：Go 版 `Toolkit` 已有 Registry、GroupManager、Executor 的分离设计，但缺少中间件、异步执行、MCP 集成、动态 schema 扩展、postprocess_func、namesake 策略等能力。

**建议行动**：
1. **中间件链**：在 `Executor` 或 `Toolkit` 中引入 middleware chain，允许在工具执行前后插入拦截器（如权限检查、日志、缓存）。
2. **异步执行**：支持标记 `AsyncExecution=true` 的工具，执行时返回 task_id，后台 goroutine 运行，Agent 可通过事件查询状态。
3. **MCP 客户端注册**：新增 `toolkit/mcp` 包，实现 `RegisterMCPClient()`，把 MCP server 的工具动态注册到 Registry 并生成 JSON Schema。
4. **PostProcess 与 Namesake 策略**：在 `Registry.Register()` 中支持：
   - `postprocess func(toolCall, toolResult) (toolResult, error)`
   - 冲突处理策略：raise / override / skip / rename
5. **元工具 `reset_equipped_tools`**：实现动态 schema 扩展，允许 Agent 在运行时通过调用该工具来激活/关闭工具组。
6. **统一返回类型**：当前 `Executor.Execute` 返回 `[]ToolResult`（自定义结构），建议改为返回 `chan *ToolResponse`（或异步生成器模式），与 Python 的 `AsyncGenerator[ToolResponse]` 对齐，支持流式工具结果。

### 8.7 状态与记忆层

**现状差距**：Go 版有独立的 `state` 包和 `memory` 包（含 ReMe、vector store、窗口等），但尚未与 Agent 的 `load_state` / `save_state` 生命周期打通。

**建议行动**：
1. **定义 AgentState 存储格式**：与 Python `AgentState` 对齐，包含 `context`、`reply_id`、`cur_iter`、`cur_summary`。
2. **在 Agent 初始化时注入 Store**：
   ```go
   type AgentStore interface {
       Load(agentName string) (*AgentState, error)
       Save(agentName string, state *AgentState) error
   }
   ```
3. **记忆压缩位点**：在 ReAct 循环的每次 reasoning 前调用压缩检查（目前 Python 也是 TODO，但接口已预留），Go 版可提前实现基于 token 数或窗口大小的压缩策略。

### 8.8 模块边界与发布策略

**启示**：Python 2.0 通过 `36a673c4` 大幅移除了 `rag`、`tts`、`realtime`、`evaluate` 等模块及其测试/示例，以减轻核心维护负担。

**建议**：Go 版目前 `rag` 包很小，`memory` 包很大（且包含大量 ReMe 专属逻辑）。建议：
- 将 `rag`、未来的 `tts`、`realtime` 等能力以 **optional plugin** 或 **子模块** 形式维护，不在核心 `agent` / `message` / `model` / `toolkit` 包中引入强依赖。
- 保持核心包的精简，有助于快速对齐 Python 2.0 的协议和接口。

---

## 9. 结论

Python 版 AgentScope 2.0 的这次重构是一次**从“消息为中心”向“事件为中心”的范式转移**。通过统一 `DataBlock`、细粒度 `AgentEvent`、统一 `Agent` 类、以及清晰的 Formatter-Model-Toolkit 边界，Python 版获得了更强的流式能力、可观测性和跨模型/跨媒介的一致性。

对于 Go 版而言，**最紧迫的对齐项**是：
1. **消息块统一为 `DataBlock`**，并调整 `ToolCallBlock` / `ToolResultBlock` 的字段；
2. **事件体系细化到块级别**，并在 Agent 流式接口中全面采用事件驱动；
3. **Agent 层引入统一状态机与挂起-恢复机制**；
4. **补齐 Formatter 抽象、Model 重试降级、Toolkit 中间件与 MCP 支持**。

只有在这些核心层面对齐后，Go 版才能与 Python 版在 A2A 协议、共享存储、Studio UI、追踪系统上实现真正的互操作。
