package middleware

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// LogFunc prints a formatted log line. Defaults to stderr when nil.
type LogFunc func(format string, args ...any)

// LoggingMiddleware logs agent lifecycle boundaries (on_reply / on_reasoning /
// on_model_call / on_acting). Suitable as a reference implementation and for
// local debugging; production deployments may prefer OTel StreamHook/recorder.
type LoggingMiddleware struct {
	Base
	Log    LogFunc
	Prefix string
}

// NewLoggingMiddleware creates a LoggingMiddleware writing to w (stderr if nil).
func NewLoggingMiddleware(w io.Writer) *LoggingMiddleware {
	if w == nil {
		w = os.Stderr
	}
	l := log.New(w, "", log.LstdFlags)
	return &LoggingMiddleware{
		Log: func(format string, args ...any) {
			l.Printf(format, args...)
		},
	}
}

func (m *LoggingMiddleware) logf(format string, args ...any) {
	if m.Log == nil {
		return
	}
	if m.Prefix != "" {
		format = m.Prefix + format
	}
	m.Log(format, args...)
}

func (m *LoggingMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	start := time.Now()
	m.logf("[middleware] on_reply start agent=%s msgs=%d", agent.AgentName(), len(input.Messages))
	msg, err := next(ctx)
	m.logf("[middleware] on_reply end agent=%s dur=%s err=%v", agent.AgentName(), time.Since(start), err)
	return msg, err
}

func (m *LoggingMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	start := time.Now()
	m.logf("[middleware] on_reasoning start agent=%s iter=%d", agent.AgentName(), input.Iteration)
	msg, err := next(ctx)
	m.logf("[middleware] on_reasoning end agent=%s iter=%d dur=%s err=%v", agent.AgentName(), input.Iteration, time.Since(start), err)
	return msg, err
}

func (m *LoggingMiddleware) OnModelCall(ctx context.Context, agent Agent, input *ModelCallInput, next ModelCallNext) (*message.Msg, error) {
	start := time.Now()
	m.logf("[middleware] on_model_call start agent=%s model=%s msgs=%d", agent.AgentName(), input.ModelName, len(input.Messages))
	msg, err := next(ctx)
	m.logf("[middleware] on_model_call end agent=%s model=%s dur=%s err=%v", agent.AgentName(), input.ModelName, time.Since(start), err)
	return msg, err
}

func (m *LoggingMiddleware) OnActing(ctx context.Context, agent Agent, input *ActingInput, next ActingNext) (*tool.Response, error) {
	start := time.Now()
	m.logf("[middleware] on_acting start agent=%s tool=%s", agent.AgentName(), input.ToolName)
	resp, err := next(ctx)
	m.logf("[middleware] on_acting end agent=%s tool=%s dur=%s err=%v", agent.AgentName(), input.ToolName, time.Since(start), err)
	return resp, err
}

func (m *LoggingMiddleware) OnSystemPrompt(ctx context.Context, agent Agent, currentPrompt string) (string, error) {
	m.logf("[middleware] on_system_prompt agent=%s len=%d", agent.AgentName(), len(currentPrompt))
	return currentPrompt, nil
}
