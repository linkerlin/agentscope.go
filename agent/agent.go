package agent

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// Agent is the core interface for all agent types
type Agent interface {
	Name() string
	Call(ctx context.Context, msg *message.Msg) (*message.Msg, error)
	CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error)
}
