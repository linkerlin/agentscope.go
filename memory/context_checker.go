package memory

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// CheckContext 按 token 阈值划分待压缩前缀与保留后缀；threshold/reserve 均为 token 估算值
func CheckContext(ctx context.Context, msgs []*message.Msg, threshold, reserve int, counter TokenCounter) (*ContextCheckResult, error) {
	_ = ctx
	if counter == nil {
		counter = NewSimpleTokenCounter()
	}
	total, err := counter.CountMessages(msgs)
	if err != nil {
		return nil, err
	}
	res := &ContextCheckResult{
		TotalTokens: total,
		Threshold: threshold,
		IsValid:   true,
	}
	if total <= threshold {
		res.MessagesToKeep = cloneMsgSlice(msgs)
		return res, nil
	}

	// 从尾部累加，直到达到 reserve tokens，确定分割点 start
	start := len(msgs)
	var acc int
	for i := len(msgs) - 1; i >= 0; i-- {
		chunk, err := counter.CountMessages([]*message.Msg{msgs[i]})
		if err != nil {
			return nil, err
		}
		acc += chunk
		start = i
		if acc >= reserve {
			break
		}
	}
	for start < len(msgs) && !splitRespectsToolPairs(msgs, start) {
		start++
	}
	res.MessagesToCompact = cloneMsgSlice(msgs[:start])
	res.MessagesToKeep = cloneMsgSlice(msgs[start:])
	res.IsValid = splitRespectsToolPairs(msgs, start)
	return res, nil
}

func cloneMsgSlice(msgs []*message.Msg) []*message.Msg {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*message.Msg, len(msgs))
	copy(out, msgs)
	return out
}

// splitRespectsToolPairs compact 前缀内每个 tool_use 在同前缀内均有对应 tool_result
func splitRespectsToolPairs(msgs []*message.Msg, start int) bool {
	if start == 0 {
		return true
	}
	pending := make(map[string]struct{})
	for _, m := range msgs[:start] {
		if m == nil {
			continue
		}
		for _, tc := range m.GetToolUseCalls() {
			pending[tc.ID] = struct{}{}
		}
		for _, tr := range m.GetToolResults() {
			delete(pending, tr.ToolUseID)
		}
	}
	return len(pending) == 0
}

