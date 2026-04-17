package interruption

import (
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// Source identifies who triggered the interruption.
type Source string

const (
	// SourceUser indicates the interruption was triggered by a user message.
	SourceUser Source = "USER"
	// SourceSystem indicates the interruption was triggered by the system
	// (e.g., graceful shutdown signal).
	SourceSystem Source = "SYSTEM"
	// SourceTool indicates the interruption was triggered by a tool or hook.
	SourceTool Source = "TOOL"
)

// Context captures metadata about an interruption, aligned with Java's
// io.agentscope.core.interruption.InterruptContext.
type Context struct {
	// Source is the origin of the interruption.
	Source Source
	// Timestamp is when the interruption occurred.
	Timestamp time.Time
	// UserMessage is the message associated with a user-driven interruption.
	UserMessage *message.Msg
	// PendingToolCalls holds any tool-use blocks that were in-flight when the
	// interruption happened.
	PendingToolCalls []*message.ToolUseBlock
}

// NewContext creates a Context with sensible defaults.
func NewContext() *Context {
	return &Context{
		Source:    SourceUser,
		Timestamp: time.Now(),
	}
}

// String returns a compact representation of the context.
func (c *Context) String() string {
	msg := "null"
	if c.UserMessage != nil {
		msg = "present"
	}
	return fmt.Sprintf(
		"InterruptContext{source=%s, timestamp=%s, userMessage=%s, pendingToolCalls=%d}",
		c.Source,
		c.Timestamp.Format(time.RFC3339),
		msg,
		len(c.PendingToolCalls),
	)
}
