package agent

import (
	"errors"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// CollectMessage drains an event stream and reconstructs the final assistant
// message by applying each event via Msg.AppendEvent.
// It returns when the channel is closed. If an ErrorEvent is encountered,
// it is returned as an error.
func CollectMessage(ch <-chan event.AgentEvent) (*message.Msg, error) {
	msg := message.NewMsg().Role(message.RoleAssistant).Build()
	var lastErr error
	for ev := range ch {
		msg.AppendEvent(ev)
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			lastErr = errors.New(e.Err)
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return msg, nil
}
