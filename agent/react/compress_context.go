package react

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/output"
)

// CompressContext compresses agent memory when token usage exceeds the trigger threshold.
// Aligned with PyV2 Agent.compress_context().
func (a *ReActAgent) CompressContext(ctx context.Context, inputMsg *message.Msg, toolSpecs []model.ToolSpec) error {
	if a == nil || a.chatModel == nil || a.memory == nil {
		return nil
	}
	if a.hasPreReasoningMemory() {
		return nil
	}
	cfg := a.contextConfig
	if cfg.TriggerRatio <= 0 || cfg.ReserveRatio <= 0 {
		cfg = agent.DefaultContextConfig()
	}
	ctxSize := a.contextSize
	if ctxSize <= 0 {
		ctxSize = model.ResolveContextSize(a.chatModel, 0)
	}

	history, err := a.buildHistory(ctx, inputMsg)
	if err != nil {
		return err
	}
	estimated, err := model.CountTokens(a.chatModel, history, toolSpecs)
	if err != nil {
		return err
	}
	threshold := int(float64(ctxSize) * cfg.TriggerRatio)
	if estimated < threshold {
		return nil
	}

	memMsgs, err := a.getMemoryMessages()
	if err != nil {
		return err
	}
	if len(memMsgs) == 0 {
		suffix := ""
		if a.getCompressedSummary() != "" {
			suffix = "and the compression summary "
		}
		return fmt.Errorf(
			"the system prompt %sexceed(s) the compression threshold (%d tokens), cannot be compressed",
			suffix, threshold,
		)
	}

	reserve := int(float64(ctxSize) * cfg.ReserveRatio)
	cc, err := memory.CheckContext(ctx, memMsgs, 0, reserve, memory.NewSimpleTokenCounter())
	if err != nil {
		return err
	}
	if len(cc.MessagesToCompact) == 0 {
		cc, err = memory.CheckContext(ctx, memMsgs, 0, 0, memory.NewSimpleTokenCounter())
		if err != nil {
			return err
		}
	}
	if len(cc.MessagesToCompact) == 0 {
		return nil
	}

	summaryText, err := a.generateCompressionSummary(ctx, cfg, cc.MessagesToCompact, toolSpecs, ctxSize)
	if err != nil {
		return err
	}
	summaryText = a.maybeOffloadCompressedContext(ctx, cc.MessagesToCompact, summaryText)
	a.setCompressedSummary(summaryText)
	if err := replaceMemory(a.memory, cc.MessagesToKeep); err != nil {
		return err
	}
	return nil
}

func (a *ReActAgent) hasPreReasoningMemory() bool {
	_, ok := a.memory.(interface {
		PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *memory.CompactSummary, error)
	})
	return ok
}

func (a *ReActAgent) getMemoryMessages() ([]*message.Msg, error) {
	if pm, ok := a.memory.(interface {
		GetMemoryForPrompt(prepend bool) ([]*message.Msg, error)
	}); ok {
		return pm.GetMemoryForPrompt(true)
	}
	return a.memory.GetAll()
}

func (a *ReActAgent) getCompressedSummary() string {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState != nil {
		return a.runtimeState.CompressedSummary
	}
	return ""
}

func (a *ReActAgent) setCompressedSummary(summary string) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState == nil {
		a.runtimeState = &agent.AgentState{}
	}
	a.runtimeState.CompressedSummary = summary
}

func (a *ReActAgent) generateCompressionSummary(
	ctx context.Context,
	cfg agent.ContextConfig,
	toCompress []*message.Msg,
	toolSpecs []model.ToolSpec,
	ctxSize int,
) (string, error) {
	prompt := cfg.CompressionPrompt
	if prompt == "" {
		prompt = agent.DefaultContextConfig().CompressionPrompt
	}
	msgs := buildCompressionMessages(a.Base.SysPrompt, a.getCompressedSummary(), toCompress, prompt)

	schema := agent.DefaultSummarySchema()
	estimated, err := model.CountTokens(a.chatModel, msgs, toolSpecs)
	if err != nil {
		return "", err
	}
	contextOverflow := estimated > ctxSize

	var summary agent.CompressionSummary
	runner := &output.StructuredRunner{Model: a.chatModel, MaxRetries: 2}
	userText := formatMessagesForCompression(msgs)
	err = runner.Run(ctx, userText, schema, &summary)
	if err != nil && contextOverflow {
		for i := 1; i <= len(toCompress); i++ {
			trimmed := buildCompressionMessages(a.Base.SysPrompt, a.getCompressedSummary(), toCompress[i:], prompt)
			est, estErr := model.CountTokens(a.chatModel, trimmed, toolSpecs)
			if estErr != nil {
				return "", estErr
			}
			if est < int(float64(ctxSize)*cfg.TriggerRatio) {
				userText = formatMessagesForCompression(trimmed)
				err = runner.Run(ctx, userText, schema, &summary)
				break
			}
		}
	}
	if err != nil {
		return "", err
	}
	return cfg.FormatCompressionSummary(summary), nil
}

func (a *ReActAgent) maybeOffloadCompressedContext(ctx context.Context, compacted []*message.Msg, summary string) string {
	if a == nil || a.offloader == nil || len(compacted) == 0 {
		return summary
	}
	data, err := json.Marshal(compacted)
	if err != nil {
		return summary
	}
	path, err := a.offloader.OffloadContext(ctx, a.sessionID(), data)
	if err != nil || path == "" {
		return summary
	}
	return summary + fmt.Sprintf(
		"\n<system-reminder>The compressed context is offloaded to '%s', you can refer to it when needed.</system-reminder>",
		path,
	)
}

func buildCompressionMessages(sysPrompt, prevSummary string, toCompress []*message.Msg, compressionPrompt string) []*message.Msg {
	var msgs []*message.Msg
	if sysPrompt != "" {
		msgs = append(msgs, message.NewMsg().Role(message.RoleSystem).TextContent(sysPrompt).Build())
	}
	if prevSummary != "" {
		msgs = append(msgs, message.NewMsg().Role(message.RoleUser).TextContent(prevSummary).Build())
	}
	msgs = append(msgs, toCompress...)
	msgs = append(msgs, message.NewMsg().Role(message.RoleUser).TextContent(compressionPrompt).Build())
	return msgs
}

func formatMessagesForCompression(msgs []*message.Msg) string {
	var b strings.Builder
	for _, m := range msgs {
		if m == nil {
			continue
		}
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.GetTextContent())
		b.WriteString("\n")
	}
	return b.String()
}

// syncHistoryWithMemory rebuilds system/summary/memory/input prefix and keeps in-loop suffix messages.
func (a *ReActAgent) syncHistoryWithMemory(ctx context.Context, inputMsg *message.Msg, history []*message.Msg) ([]*message.Msg, error) {
	base, err := a.buildHistory(ctx, inputMsg)
	if err != nil {
		return nil, err
	}
	suffix := inLoopSuffixAfterInput(history, inputMsg)
	if len(suffix) == 0 {
		return base, nil
	}
	return append(base, suffix...), nil
}

func inLoopSuffixAfterInput(history []*message.Msg, input *message.Msg) []*message.Msg {
	if input == nil || len(history) == 0 {
		return nil
	}
	for i, m := range history {
		if m != nil && m.ID == input.ID {
			if i+1 < len(history) {
				return append([]*message.Msg(nil), history[i+1:]...)
			}
			return nil
		}
	}
	return nil
}

func replaceMemory(m memory.Memory, msgs []*message.Msg) error {
	if m == nil {
		return errors.New("react: nil memory")
	}
	if err := m.Clear(); err != nil {
		return err
	}
	for _, msg := range msgs {
		if err := m.Add(msg); err != nil {
			return err
		}
	}
	return nil
}
