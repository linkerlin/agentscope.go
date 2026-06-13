# AgentScope.Go × LlamaIndex 桥接

将 AgentScope.Go Agent 作为 LlamaIndex 的 Query Engine 或 Tool 使用，实现跨框架互操作。

## 快速开始

### Python 端（LlamaIndex）

```python
from llama_index.core.tools import FunctionTool
from llama_index.core.agent import ReActAgent
from llama_index.llms.openai import OpenAI
import requests
import json

class AgentScopeGoQueryEngine:
    """将 AgentScope.Go 服务封装为 LlamaIndex Query Engine"""
    
    def __init__(self, base_url: str = "http://localhost:8080", api_key: str = ""):
        self.base_url = base_url
        self.api_key = api_key
        self.agent_id = "demo-agent-1"
    
    def query(self, query_str: str) -> str:
        """执行查询"""
        resp = requests.post(
            f"{self.base_url}/v2/chat",
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.api_key}",
            },
            json={
                "agent_id": self.agent_id,
                "input": {"text": query_str}
            },
            timeout=60,
        )
        resp.raise_for_status()
        data = resp.json()
        events = data.get("events", [])
        texts = []
        for evt in events:
            if evt.get("type") in ("text_block_delta", "TEXT_MESSAGE_CONTENT"):
                texts.append(evt.get("delta", ""))
        return "".join(texts) if texts else json.dumps(data)
    
    async def aquery(self, query_str: str) -> str:
        """异步查询"""
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
                    "input": {"text": query_str}
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

# 封装为 LlamaIndex Tool
def agentscope_query(query: str) -> str:
    engine = AgentScopeGoQueryEngine(base_url="http://localhost:8080", api_key="your-key")
    return engine.query(query)

tool = FunctionTool.from_defaults(
    fn=agentscope_query,
    name="agentscope_go",
    description="使用 AgentScope.Go 执行复杂 Agent 任务",
)

# 使用示例
llm = OpenAI(model="gpt-4o-mini")
agent = ReActAgent.from_tools([tool], llm=llm, verbose=True)

response = agent.chat("使用 AgentScope.Go 工具帮我总结今天的日程")
print(response)
```

## RAG 集成

### 将 ReMe 记忆作为 LlamaIndex 索引源

```python
from llama_index.core import Document, VectorStoreIndex

# 从 AgentScope.Go ReMe 检索记忆
def fetch_reme_memories(query: str, top_k: int = 5) -> list[Document]:
    resp = requests.get(
        f"{base_url}/api/studio/memory/search",
        headers={"Authorization": f"Bearer {api_key}"},
        params={"q": query, "top_k": top_k},
    )
    data = resp.json()
    documents = []
    for node in data.get("results", []):
        documents.append(Document(
            text=node.get("content", ""),
            metadata={
                "memory_type": node.get("memory_type"),
                "score": node.get("score"),
                "author": node.get("author"),
            }
        ))
    return documents

# 构建 LlamaIndex 索引
memories = fetch_reme_memories("项目经验")
index = VectorStoreIndex.from_documents(memories)
query_engine = index.as_query_engine()

response = query_engine.query("我在这个项目中学到了什么？")
print(response)
```

## 多 Agent 编排桥接

```python
from llama_index.core.tools import QueryEngineTool

# 将 AgentScope.Go 的 Workflow 编排暴露为 LlamaIndex Query Engine Tool
pipeline_engine = AgentScopeGoQueryEngine(
    base_url="http://localhost:8080",
    api_key="your-key",
)
pipeline_tool = QueryEngineTool.from_defaults(
    query_engine=pipeline_engine,
    name="agentscope_pipeline",
    description="执行 AgentScope.Go 多 Agent 编排管道",
)

agent = ReActAgent.from_tools([tool, pipeline_tool], llm=llm, verbose=True)
```

## 安全注意事项

- 使用 API Key 或 JWT 进行认证
- 生产环境使用 HTTPS
- 限制 AgentScope.Go 工具的权限范围（RBAC）
- 敏感操作启用 HITL 确认

## 更多资源

- [LlamaIndex 文档](https://docs.llamaindex.ai/)
- AgentScope.Go API 参考：`docs/api-reference.md`
