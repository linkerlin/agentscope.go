// Package middleware provides Agent-level lifecycle interceptors aligned with
// Python v2 MiddlewareBase (on_reply / on_reasoning / on_check_permission /
// on_acting / on_model_call / on_compress_context / on_system_prompt).
package middleware

import (
	"context"
	"errors"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// Agent is the minimal agent view passed to middleware implementations.
type Agent interface {
	AgentName() string
}

// Middleware is a marker interface for agent lifecycle middleware.
type Middleware interface {
	middleware()
}

// Base can be embedded by custom middleware types.
type Base struct{}

func (Base) middleware() {}

// ReplyInput carries inputs for on_reply middleware.
type ReplyInput struct {
	Messages []*message.Msg
}

// ErrContinueReply is a sentinel an on_reply middleware may return instead of
// a final answer: the agent re-runs the reply (with the previous reply
// appended to the input messages) so reasoning can continue. The loop is
// bounded (see Base.Call); after the bound the last reply wins.
// PyV2 #2322 parity (on_reply middlewares swallowing ReplyEndEvent).
var ErrContinueReply = errors.New("middleware: continue reply loop")

// ReplyNext executes the next middleware or the core reply handler.
type ReplyNext func(ctx context.Context) (*message.Msg, error)

// ReplyInterceptor wraps the entire reply lifecycle (on_reply).
type ReplyInterceptor interface {
	Middleware
	OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error)
}

// ReasoningInput carries inputs for on_reasoning middleware.
type ReasoningInput struct {
	Iteration int
	Messages  []*message.Msg
	ChatOpts  []model.ChatOption
}

// ReasoningNext executes the next middleware or core reasoning step.
type ReasoningNext func(ctx context.Context) (*message.Msg, error)

// ReasoningInterceptor wraps a single reasoning step (on_reasoning).
type ReasoningInterceptor interface {
	Middleware
	OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error)
}

// ActingInput carries inputs for on_acting middleware (raw tool I/O only).
type ActingInput struct {
	ToolName  string
	ToolInput map[string]any
}

// ActingNext executes the next middleware or raw tool execution.
type ActingNext func(ctx context.Context) (*tool.Response, error)

// ActingInterceptor wraps raw toolkit.call_tool equivalent (on_acting).
type ActingInterceptor interface {
	Middleware
	OnActing(ctx context.Context, agent Agent, input *ActingInput, next ActingNext) (*tool.Response, error)
}

// ModelCallInput carries inputs for on_model_call middleware.
type ModelCallInput struct {
	Messages  []*message.Msg
	ChatOpts  []model.ChatOption
	ModelName string
}

// ModelCallNext executes the next middleware or the raw model API call.
type ModelCallNext func(ctx context.Context) (*message.Msg, error)

// ModelCallInterceptor wraps the raw model Chat/ChatStream call (on_model_call).
type ModelCallInterceptor interface {
	Middleware
	OnModelCall(ctx context.Context, agent Agent, input *ModelCallInput, next ModelCallNext) (*message.Msg, error)
}

// SystemPromptTransformer transforms system prompt strings (pipeline, not onion).
type SystemPromptTransformer interface {
	Middleware
	OnSystemPrompt(ctx context.Context, agent Agent, currentPrompt string) (string, error)
}

// PermissionResult is the outcome of a permission check, mirroring
// permission.Result. A local type avoids a circular import
// (permission tests → skill → toolkit → agent → middleware → permission).
type PermissionResult struct {
	ToolCallID string
	ToolName   string
	Decision   string // "allow" | "deny" | "ask" | "passthrough"
	Message    string
	Reason     string
}

// PermissionInput carries inputs for on_check_permission middleware.
type PermissionInput struct {
	ToolCallID string
	ToolName   string
	ToolInput  map[string]any
}

// PermissionNext executes the next middleware or the core permission engine check.
type PermissionNext func(ctx context.Context) (PermissionResult, error)

// PermissionInterceptor wraps the permission check for a tool call (on_check_permission).
// A middleware can inspect, audit, replace, or bypass the permission decision.
type PermissionInterceptor interface {
	Middleware
	OnCheckPermission(ctx context.Context, agent Agent, input *PermissionInput, next PermissionNext) (PermissionResult, error)
}

// CompressionInput carries inputs for on_compress_context middleware.
type CompressionInput struct {
	Messages []*message.Msg
}

// CompressionNext executes the next middleware or the core context compression.
type CompressionNext func(ctx context.Context) error

// CompressionInterceptor wraps context compression (on_compress_context).
type CompressionInterceptor interface {
	Middleware
	OnCompressContext(ctx context.Context, agent Agent, input *CompressionInput, next CompressionNext) error
}
