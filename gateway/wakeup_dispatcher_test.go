package gateway

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// fakeV2Agent is a no-op V2Agent whose ReplyStream closes immediately,
// letting SessionManager.Run finish instantly in tests. (The dispatcher
// behaviour tests moved to gateway/wakeupapi in 16.2.)
type fakeV2Agent struct{}

func (f *fakeV2Agent) Name() string { return "fake" }
func (f *fakeV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return msg, nil
}
func (f *fakeV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg)
	close(ch)
	return ch, nil
}
func (f *fakeV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return msg, nil
}
func (f *fakeV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	close(ch)
	return ch, nil
}
func (f *fakeV2Agent) LoadState(s *agent.AgentState) error   { return nil }
func (f *fakeV2Agent) SaveState() (*agent.AgentState, error) { return &agent.AgentState{}, nil }
func (f *fakeV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}
