package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

// RAGHook 在 BeforeModel 阶段将检索结果注入 system message
type RAGHook struct {
	Retriever      MemoryRAG
	TopK           int
	PromptTemplate string
}

// NewRAGHook 创建 RAG Hook
func NewRAGHook(retriever MemoryRAG, topK int) *RAGHook {
	return &RAGHook{
		Retriever:      retriever,
		TopK:           topK,
		PromptTemplate: "## Relevant Context\n\n%s\n",
	}
}

// OnEvent 实现 hook.Hook 接口
func (h *RAGHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if hCtx.Point != hook.HookBeforeModel {
		return nil, nil
	}
	query := extractLastUserQuery(hCtx.Messages)
	if query == "" || h.Retriever == nil {
		return nil, nil
	}
	topK := h.TopK
	if topK <= 0 {
		topK = 3
	}
	msgs, err := h.Retriever.Retrieve(ctx, query, topK)
	if err != nil || len(msgs) == 0 {
		return nil, nil
	}
	var parts []string
	for _, m := range msgs {
		text := m.GetTextContent()
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	contextText := strings.Join(parts, "\n\n")
	prompt := h.PromptTemplate
	if prompt == "" {
		prompt = "## Relevant Context\n\n%s\n"
	}
	injected := fmt.Sprintf(prompt, contextText)

	sysIdx := -1
	for i, m := range hCtx.Messages {
		if m.Role == message.RoleSystem {
			sysIdx = i
			break
		}
	}

	if sysIdx >= 0 {
		newMsg := message.NewMsg().Role(message.RoleSystem).
			TextContent(hCtx.Messages[sysIdx].GetTextContent() + "\n\n" + injected).Build()
		newMsgs := make([]*message.Msg, len(hCtx.Messages))
		copy(newMsgs, hCtx.Messages)
		newMsgs[sysIdx] = newMsg
		return &hook.HookResult{InjectMessages: newMsgs}, nil
	}

	newMsg := message.NewMsg().Role(message.RoleSystem).TextContent(injected).Build()
	newMsgs := append([]*message.Msg{newMsg}, hCtx.Messages...)
	return &hook.HookResult{InjectMessages: newMsgs}, nil
}

func extractLastUserQuery(msgs []*message.Msg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.RoleUser {
			return msgs[i].GetTextContent()
		}
	}
	return ""
}
