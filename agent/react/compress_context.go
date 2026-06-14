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

	if prev := a.getCompressedSummary(); prev != "" {
		summaryText = mergeCompressionSummaries(prev, summaryText)
	}

	summaryText = a.maybeMetaCompressSummary(ctx, cfg, summaryText, toolSpecs, ctxSize)

	summaryText = a.maybeOffloadCompressedContext(ctx, cc.MessagesToCompact, summaryText)
	a.setCompressedSummary(summaryText)
	a.addCompressionWatermark(len(cc.MessagesToCompact))
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
	toCompress = preTruncateToolResults(a.chatModel, toCompress, cfg.ToolResultLimit)
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

// --- Incremental Compression Enhancement ---

func (a *ReActAgent) getCompressionWatermark() int {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState != nil {
		return a.runtimeState.CompressionWatermark
	}
	return 0
}

func (a *ReActAgent) addCompressionWatermark(delta int) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState == nil {
		a.runtimeState = &agent.AgentState{}
	}
	a.runtimeState.CompressionWatermark += delta
}

// preTruncateToolResults shrinks oversized tool results in messages before compression.
// This reduces the token cost of the summarization LLM call.
func preTruncateToolResults(m model.ChatModel, msgs []*message.Msg, limit int) []*message.Msg {
	if m == nil || limit <= 0 {
		return msgs
	}
	out := make([]*message.Msg, len(msgs))
	for i, msg := range msgs {
		if msg == nil {
			out[i] = msg
			continue
		}
		var changed bool
		var blocks []message.ContentBlock
		for _, b := range msg.Content {
			if tr, ok := b.(*message.ToolResultBlock); ok && tr != nil {
				reserved, offload, err := SplitToolResultForCompression(m, tr, limit)
				if err == nil && offload != nil && reserved != nil {
					blocks = append(blocks, reserved)
					changed = true
					continue
				}
			}
			blocks = append(blocks, b)
		}
		if changed {
			cloned := *msg
			cloned.Content = blocks
			out[i] = &cloned
		} else {
			out[i] = msg
		}
	}
	return out
}

// mergeCompressionSummaries merges a previous summary text with a new delta summary text.
// Both are rendered from the SummaryTemplate, so we use heuristic field-level merge.
func mergeCompressionSummaries(prevText, deltaText string) string {
	prev := parseSummaryFields(prevText)
	delta := parseSummaryFields(deltaText)

	merged := agent.CompressionSummary{
		TaskOverview:         preferNonEmpty(delta.TaskOverview, prev.TaskOverview),
		CurrentState:         preferNonEmpty(delta.CurrentState, prev.CurrentState),
		ImportantDiscoveries: mergeTexts(prev.ImportantDiscoveries, delta.ImportantDiscoveries),
		NextSteps:            preferNonEmpty(delta.NextSteps, prev.NextSteps),
		ContextToPreserve:    mergeTexts(prev.ContextToPreserve, delta.ContextToPreserve),
	}
	cfg := agent.DefaultContextConfig()
	return cfg.FormatCompressionSummary(merged)
}

func (a *ReActAgent) maybeMetaCompressSummary(
	ctx context.Context,
	cfg agent.ContextConfig,
	summaryText string,
	toolSpecs []model.ToolSpec,
	ctxSize int,
) string {
	ratio := cfg.SummaryTokenRatio
	if ratio <= 0 {
		ratio = 0.15
	}
	cap := int(float64(ctxSize) * ratio)
	if cap <= 0 || len(summaryText) <= cap {
		return summaryText
	}
	if a.chatModel == nil {
		return truncateSummaryText(summaryText, cap)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(
			"You are a context compressor. Given a verbose summary, produce a concise version " +
				"that retains all critical information. Keep the same structured format.").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(summaryText +
			"\n\nCompress this summary to be as short as possible while keeping all key facts.").Build(),
	}
	est, err := model.CountTokens(a.chatModel, msgs, nil)
	if err != nil || est > ctxSize {
		return truncateSummaryText(summaryText, cap)
	}
	resp, err := a.chatModel.Chat(ctx, msgs, nil)
	if err != nil || resp == nil {
		return truncateSummaryText(summaryText, cap)
	}
	text := resp.GetTextContent()
	if text == "" {
		return summaryText
	}
	return text
}

func truncateSummaryText(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	marker := "\n<<<TRUNCATED>>>"
	cut := maxLen - len(marker)
	if cut < 0 {
		cut = 0
	}
	if cut > len(s) {
		cut = len(s)
	}
	return s[:cut] + marker
}

// parseSummaryFields extracts CompressionSummary fields from rendered template text.
// Uses section headers from DefaultContextConfig().SummaryTemplate.
func parseSummaryFields(text string) agent.CompressionSummary {
	var s agent.CompressionSummary
	s.TaskOverview = extractSection(text, "# Task Overview")
	s.CurrentState = extractSection(text, "# Current State")
	s.ImportantDiscoveries = extractSection(text, "# Important Discoveries")
	s.NextSteps = extractSection(text, "# Next Steps")
	s.ContextToPreserve = extractSection(text, "# Context to Preserve")
	return s
}

func extractSection(text, header string) string {
	idx := strings.Index(text, header)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(header):]
	nextSection := len(rest)
	for _, h := range []string{"# Task Overview", "# Current State", "# Important Discoveries", "# Next Steps", "# Context to Preserve", "</system-info>"} {
		if i := strings.Index(rest, h); i >= 0 && i < nextSection {
			nextSection = i
		}
	}
	return strings.TrimSpace(rest[:nextSection])
}

func preferNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func mergeTexts(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if strings.Contains(a, b) {
		return a
	}
	return a + "\n" + b
}
