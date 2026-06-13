# AgentScope.Go × LangChain 桥接

将 AgentScope.Go Agent 作为 LangChain Tool 使用，实现跨框架互操作。

## 快速开始

### Python 端（LangChain）

```python
from langchain.agents import AgentExecutor, create_react_agent
from langchain.tools import BaseTool
from langchain_openai import ChatOpenAI
import requests
import json

class AgentScopeGoTool(BaseTool):
    """将 AgentScope.Go Agent 封装为 LangChain Tool"""
    name: str = "agentscope_go"
    description: str = "调用 AgentScope.Go 服务执行复杂任务"
    base_url: str = "http://localhost:8080"
    api_key: str = ""
    agent_id: str = "demo-agent-1"
    
    def _run(self, query: str) -> str:
        """同步调用"""
        resp = requests.post(
            f"{self.base_url}/v2/chat",
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.api_key}",
            },
            json={
                "agent_id": self.agent_id,
                "input": {"text": query}
            },
            timeout=60,
        )
        resp.raise_for_status()
        data = resp.json()
        # 从事件流中提取文本
        events = data.get("events", [])
        texts = []
        for evt in events:
            if evt.get("type") in ("text_block_delta", "TEXT_MESSAGE_CONTENT"):
                texts.append(evt.get("delta", ""))
        return "".join(texts) if texts else json.dumps(data)
    
    async def _arun(self, query: str) -> str:
        """异步调用（使用 SSE 流式）"""
        import aiohttp
        async with aiohttp.ClientSession() as session:
            async with session.post(
                f"{self.base_url}/v2/chat/stream",
                headers={
                    "Content-Type": "application/json",
                    "Authorization": f"Bearer {self.api_key}",
                },
                json={
                    "agent_id": self.agent_id,
                    "input": {"text": query}
                },
            ) as resp:
                texts = []
                async for line in resp.content:
                    line = line.decode().strip()
                    if line.startswith("data:"):
                        try:
                            evt = json.loads(line[5:])
                            if evt.get("type") in ("text_block_delta", "TEXT_MESSAGE_CONTENT"):
                                texts.append(evt.get("delta", ""))
                        except:
                            pass
                return "".join(texts)

# 使用示例
llm = ChatOpenAI(model="gpt-4o-mini")
tools = [AgentScopeGoTool(base_url="http://localhost:8080", api_key="your-key")]
agent = create_react_agent(llm, tools)
executor = AgentExecutor(agent=agent, tools=tools, verbose=True)

result = executor.invoke({"input": "使用 AgentScope.Go 工具查询我的日程安排"})
print(result)
```

## 高级用法

### 多 Agent 编排桥接

```python
from langchain.tools import StructuredTool

# 将 AgentScope.Go 的 Pipeline 编排暴露为 LangChain 工具
def run_pipeline(task: str, pipeline_name: str = "default") -> str:
    resp = requests.post(
        f"{base_url}/api/v1/pipelines/{pipeline_name}/run",
        headers={"Authorization": f"Bearer {api_key}"},
        json={"input": task},
    )
    return resp.json().get("output", "")

pipeline_tool = StructuredTool.from_function(
    func=run_pipeline,
    name="agentscope_pipeline",
    description="执行 AgentScope.Go 多 Agent 编排管道",
)
```

### 记忆共享

```python
# 将 LangChain 的聊天记录同步到 AgentScope.Go ReMe 记忆
from langchain.memory import ConversationBufferMemory

class ReMeMemory(ConversationBufferMemory):
    """将 LangChain 记忆同步到 AgentScope.Go ReMe"""
    
    def save_context(self, inputs, outputs):
        super().save_context(inputs, outputs)
        # 同步到 ReMe
        requests.post(
            f"{self.base_url}/api/studio/memory/add",
            headers={"Authorization": f"Bearer {self.api_key}"},
            json={"text": f"User: {inputs}\nAssistant: {outputs}"},
        )
```

## 安全注意事项

- 使用 API Key 或 JWT 进行认证
- 生产环境使用 HTTPS
- 限制 AgentScope.Go 工具的权限范围（RBAC）
- 敏感操作启用 HITL 确认

## 更多资源

- [LangChain 文档](https://python.langchain.com/)
- AgentScope.Go API 参考：`docs/api-reference.md`
