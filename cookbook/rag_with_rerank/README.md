# RAG with Rerank

This cookbook demonstrates a two-stage retrieval pipeline in AgentScope.Go:

1. **Vector retrieval** recalls the top candidate documents using embeddings.
2. **Reranking** reorders the candidates by relevance to the user query using a
   dedicated reranker.

## Reranker Backends

The example automatically picks a reranker in this order:

- **Cohere Rerank v2** if `COHERE_API_KEY` is set.
- **Jina Rerank** if `JINA_API_KEY` is set.
- **Local cosine-similarity reranker** as a fallback (uses the same embedding
  model as retrieval).

## Prerequisites

- Go 1.24+
- An OpenAI API key for embeddings and chat completion.
- Optionally a Cohere or Jina API key for cloud reranking.

## Run

```bash
export OPENAI_API_KEY="sk-..."
# Optional:
export COHERE_API_KEY="..."
# or
export JINA_API_KEY="..."

go run ./cookbook/rag_with_rerank
```

The program indexes a few small documents, retrieves results with and without
reranking, and then answers the user question using reranked context.
