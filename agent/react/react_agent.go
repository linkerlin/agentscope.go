package react

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

const defaultMaxIterations = 10

// ReActAgent implements the ReAct (Reasoning + Acting) pattern
type ReActAgent struct {
	name          string
	sysPrompt     string
	chatModel     model.ChatModel
	tools         []tool.Tool
	memory        memory.Memory
	maxIterations int
	hooks         []hook.Hook
	toolMap       map[string]tool.Tool
}

// ReActAgentBuilder provides a fluent API for constructing ReActAgent
type ReActAgentBuilder struct {
	name          string
	sysPrompt     string
	chatModel     model.ChatModel
	tools         []tool.Tool
	memory        memory.Memory
	maxIterations int
	hooks         []hook.Hook
}

// Builder returns a new ReActAgentBuilder
func Builder() *ReActAgentBuilder {
	return &ReActAgentBuilder{
		maxIterations: defaultMaxIterations,
	}
}

func (b *ReActAgentBuilder) Name(name string) *ReActAgentBuilder {
	b.name = name
	return b
}

func (b *ReActAgentBuilder) SysPrompt(prompt string) *ReActAgentBuilder {
	b.sysPrompt = prompt
	return b
}

func (b *ReActAgentBuilder) Model(m model.ChatModel) *ReActAgentBuilder {
	b.chatModel = m
	return b
}

func (b *ReActAgentBuilder) Tools(tools ...tool.Tool) *ReActAgentBuilder {
	b.tools = append(b.tools, tools...)
	return b
}

func (b *ReActAgentBuilder) Memory(mem memory.Memory) *ReActAgentBuilder {
	b.memory = mem
	return b
}

func (b *ReActAgentBuilder) MaxIterations(n int) *ReActAgentBuilder {
	b.maxIterations = n
	return b
}

func (b *ReActAgentBuilder) Hooks(hooks ...hook.Hook) *ReActAgentBuilder {
	b.hooks = append(b.hooks, hooks...)
	return b
}

func (b *ReActAgentBuilder) Build() (*ReActAgent, error) {
	if b.name == "" {
		return nil, errors.New("react agent: name is required")
	}
	if b.chatModel == nil {
		return nil, errors.New("react agent: model is required")
	}
	if b.memory == nil {
		b.memory = memory.NewInMemoryMemory()
	}

	toolMap := make(map[string]tool.Tool, len(b.tools))
	for _, t := range b.tools {
		toolMap[t.Name()] = t
	}

	return &ReActAgent{
		name:          b.name,
		sysPrompt:     b.sysPrompt,
		chatModel:     b.chatModel,
		tools:         b.tools,
		memory:        b.memory,
		maxIterations: b.maxIterations,
		hooks:         b.hooks,
		toolMap:       toolMap,
	}, nil
}

func (a *ReActAgent) Name() string { return a.name }

// Call executes the ReAct loop and returns the final response
func (a *ReActAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	// Build initial message history
	history, err := a.buildHistory(msg)
	if err != nil {
		return nil, err
	}

	toolSpecs := a.toolSpecs()
	var chatOpts []model.ChatOption
	if len(toolSpecs) > 0 {
		chatOpts = append(chatOpts, model.WithTools(toolSpecs))
	}

	var finalResponse *message.Msg

	for i := 0; i < a.maxIterations; i++ {
		// Fire before-model hooks
		if hr, err := a.fireHooks(ctx, hook.HookBeforeModel, history, nil, "", nil); err != nil {
			return nil, err
		} else if hr != nil && hr.Interrupt {
			return hr.Override, nil
		} else if hr != nil && hr.Override != nil {
			finalResponse = hr.Override
			break
		}

		// Call the model
		response, err := a.chatModel.Chat(ctx, history, chatOpts...)
		if err != nil {
			return nil, fmt.Errorf("react agent model call: %w", err)
		}

		// Fire after-model hooks
		if hr, err := a.fireHooks(ctx, hook.HookAfterModel, history, response, "", nil); err != nil {
			return nil, err
		} else if hr != nil && hr.Interrupt {
			return hr.Override, nil
		} else if hr != nil && hr.Override != nil {
			response = hr.Override
		}

		history = append(history, response)

		toolCalls := response.GetToolUseCalls()
		if len(toolCalls) == 0 {
			// No tool calls - this is the final answer
			if hr, err := a.fireHooks(ctx, hook.HookBeforeFinish, history, response, "", nil); err != nil {
				return nil, err
			} else if hr != nil && hr.Override != nil {
				response = hr.Override
			}
			finalResponse = response
			break
		}

		// Execute tool calls
		toolResultMsg := message.NewMsg().Role(message.RoleTool)
		for _, tc := range toolCalls {
			// Fire before-tool hook
			if hr, err := a.fireHooks(ctx, hook.HookBeforeTool, history, nil, tc.Name, tc.Input); err != nil {
				return nil, err
			} else if hr != nil && hr.Interrupt {
				return hr.Override, nil
			}

			result, toolErr := a.executeTool(ctx, tc.Name, tc.Input)

			// Fire after-tool hook
			afterHr, afterErr := a.fireHooks(ctx, hook.HookAfterTool, history, nil, tc.Name, tc.Input)
			if afterErr != nil {
				return nil, afterErr
			}
			if afterHr != nil && afterHr.Interrupt {
				return afterHr.Override, nil
			}

			var resultText string
			if toolErr != nil {
				resultText = fmt.Sprintf("error: %s", toolErr.Error())
				toolResultMsg.Content(message.NewToolResultBlock(tc.ID, []message.ContentBlock{
					message.NewTextBlock(resultText),
				}, true))
			} else {
				resultJSON, _ := json.Marshal(result)
				resultText = string(resultJSON)
				toolResultMsg.Content(message.NewToolResultBlock(tc.ID, []message.ContentBlock{
					message.NewTextBlock(resultText),
				}, false))
			}
		}
		history = append(history, toolResultMsg.Build())
	}

	if finalResponse == nil {
		return nil, errors.New("react agent: max iterations reached without final answer")
	}

	// Persist to memory
	_ = a.memory.Add(msg)
	_ = a.memory.Add(finalResponse)

	return finalResponse, nil
}

// CallStream executes the ReAct loop with streaming output
func (a *ReActAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 16)
	go func() {
		defer close(ch)
		resp, err := a.Call(ctx, msg)
		if err != nil {
			// Send error message
			ch <- message.NewMsg().
				Role(message.RoleAssistant).
				TextContent(fmt.Sprintf("error: %s", err.Error())).
				Build()
			return
		}
		ch <- resp
	}()
	return ch, nil
}

// buildHistory assembles system prompt + memory + new user message
func (a *ReActAgent) buildHistory(userMsg *message.Msg) ([]*message.Msg, error) {
	var history []*message.Msg

	if a.sysPrompt != "" {
		history = append(history, message.NewMsg().
			Role(message.RoleSystem).
			TextContent(a.sysPrompt).
			Build())
	}

	memMsgs, err := a.memory.GetAll()
	if err != nil {
		return nil, err
	}
	history = append(history, memMsgs...)
	history = append(history, userMsg)
	return history, nil
}

// toolSpecs converts tools to model.ToolSpec slice
func (a *ReActAgent) toolSpecs() []model.ToolSpec {
	specs := make([]model.ToolSpec, 0, len(a.tools))
	for _, t := range a.tools {
		specs = append(specs, t.Spec())
	}
	return specs
}

// executeTool finds and runs the named tool
func (a *ReActAgent) executeTool(ctx context.Context, name string, input map[string]any) (any, error) {
	t, ok := a.toolMap[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return t.Execute(ctx, input)
}

// fireHooks fires all registered hooks for the given point
func (a *ReActAgent) fireHooks(
	ctx context.Context,
	point hook.HookPoint,
	messages []*message.Msg,
	response *message.Msg,
	toolName string,
	toolInput map[string]any,
) (*hook.HookResult, error) {
	if len(a.hooks) == 0 {
		return nil, nil
	}
	hCtx := &hook.HookContext{
		AgentName: a.name,
		Point:     point,
		Messages:  messages,
		Response:  response,
		ToolName:  toolName,
		ToolInput: toolInput,
		Metadata:  make(map[string]any),
	}
	for _, h := range a.hooks {
		result, err := h.OnEvent(ctx, hCtx)
		if err != nil {
			return nil, err
		}
		if result != nil && (result.Interrupt || result.Override != nil) {
			return result, nil
		}
	}
	return nil, nil
}

// Ensure ReActAgent satisfies agent.Agent (compile-time check via blank import in agent package)
var _ interface {
	Name() string
	Call(ctx context.Context, msg *message.Msg) (*message.Msg, error)
	CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error)
} = (*ReActAgent)(nil)
