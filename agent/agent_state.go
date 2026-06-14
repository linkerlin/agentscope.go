package agent

import (
	"encoding/json"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// AgentState is a serializable snapshot of an agent's runtime state.
// It enables suspend-resume across HTTP requests and crash recovery.
type AgentState struct {
	Version   string `json:"version"`
	ReplyID   string `json:"reply_id"`
	CurIter   int    `json:"cur_iter"`
	MaxIters  int    `json:"max_iters"`
	AgentName string `json:"agent_name"`
	AgentID   string `json:"agent_id"`

	// Context
	Messages             []*message.Msg `json:"messages"`
	CompressedSummary    string         `json:"compressed_summary,omitempty"`
	CompressionWatermark int            `json:"compression_watermark,omitempty"`

	// Tool context
	ToolContext ToolContext `json:"tool_context,omitempty"`

	// Permission context
	PermissionContext PermissionContext `json:"permission_context,omitempty"`

	// Suspend state (critical for HITL)
	SuspendedAt    *time.Time `json:"suspended_at,omitempty"`
	SuspendedEvent string     `json:"suspended_event,omitempty"`
	WaitConfirmID  string     `json:"wait_confirm_id,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToolContext holds the tool-related runtime state.
type ToolContext struct {
	EquippedGroups []string                   `json:"equipped_groups,omitempty"`
	PendingCalls   []PendingToolCall          `json:"pending_calls,omitempty"`
	Results        map[string]ToolResultState `json:"results,omitempty"`
}

// PendingToolCall represents a tool call that is in-flight or awaiting confirmation.
type PendingToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Args  json.RawMessage `json:"args"`
	State string          `json:"state"` // pending | asking | allowed | submitted | finished
}

// ToolResultState represents the outcome of a tool execution.
type ToolResultState struct {
	ToolCallID string `json:"tool_call_id"`
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// PermissionContext holds the permission runtime state.
type PermissionContext struct {
	Mode  string           `json:"mode,omitempty"` // explore | accept_edits | bypass | dont_ask
	Rules []PermissionRule `json:"rules,omitempty"`
}

// PermissionRule is a single permission rule.
type PermissionRule struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	Pattern  string `json:"pattern"`
	Decision string `json:"decision"` // allow | deny | ask | passthrough
}

// StateType implements state.State (for compatibility with existing store layer).
func (s AgentState) StateType() string { return "agent_runtime_state" }
