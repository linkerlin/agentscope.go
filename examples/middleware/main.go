// Example: agent-level Middleware + HookOnError (no source changes to AgentScope core).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
)

type mockModel struct{ fail bool }

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.fail {
		return nil, fmt.Errorf("mock model unavailable")
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "pong"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockModel) ModelName() string { return "mock" }

func main() {
	logMW := middleware.NewLoggingMiddleware(os.Stderr)
	logMW.Prefix = "[demo] "

	errorHook := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point != hook.HookOnError {
			return nil, nil
		}
		fmt.Fprintf(os.Stderr, "[demo] on_error agent=%s err=%v\n", hCtx.AgentName, hCtx.Err)
		return nil, nil
	})

	agentOK, err := react.Builder().
		Name("MiddlewareDemo").
		SysPrompt("You are helpful.").
		Model(&mockModel{}).
		Middlewares(logMW).
		Hooks(errorHook).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agentOK.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("ping").Build())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("reply:", resp.GetTextContent())

	agentFail, err := react.Builder().
		Name("MiddlewareDemo").
		Model(&mockModel{fail: true}).
		Middlewares(logMW).
		Hooks(errorHook).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := agentFail.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("ping").Build()); err == nil {
		log.Fatal("expected model error")
	}
	fmt.Println("model error surfaced as expected (see on_error log above)")
}
