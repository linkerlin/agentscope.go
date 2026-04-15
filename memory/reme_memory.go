package memory

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// ReMeMemory 在 Memory 之上扩展上下文检查、压缩、持久化与 token 估计
type ReMeMemory interface {
	Memory

	CheckContext(ctx context.Context, threshold, reserve int) (*ContextCheckResult, error)
	CompactMemory(ctx context.Context, messages []*message.Msg, opts CompactOptions) (string, error)
	EstimateTokens(messages []*message.Msg) (*TokenStats, error)
	SaveTo(sessionID string) error
	LoadFrom(sessionID string) error

	// PreReasoningPrepare 在模型调用前准备消息视图（供 Hook 注入等）
	PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *CompactSummary, error)
}

// Orchestrator 记忆编排器接口，由外部（如 handler 包）实现并注入 ReMeVectorMemory
type Orchestrator interface {
	Summarize(ctx context.Context, msgs []*message.Msg, userName, taskName, toolName string) (*SummarizeResult, error)
	Retrieve(ctx context.Context, query string, userName, taskName, toolName string, opts RetrieveOptions) ([]*MemoryNode, error)
	AddToolCallResult(result ToolCallResult) error
	SummarizeToolUsage(ctx context.Context, toolName string) error
}
