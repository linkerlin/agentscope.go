package message

// MsgRole represents the role of a message sender
type MsgRole string

const (
	RoleUser      MsgRole = "user"
	RoleAssistant MsgRole = "assistant"
	RoleSystem    MsgRole = "system"
	RoleTool      MsgRole = "tool"
)
