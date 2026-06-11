package react

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

const toolResultTruncationReminder = "\n<<<TRUNCATED>>>\n<system-reminder>The remaining content has been omitted for limited context.%s</system-reminder>"

// SplitToolResultForCompression splits an oversized tool result into reserved and offload parts.
// Aligned with PyV2 Agent._split_tool_result_for_compression().
func SplitToolResultForCompression(m model.ChatModel, block *message.ToolResultBlock, limit int) (*message.ToolResultBlock, *message.ToolResultBlock, error) {
	if block == nil || limit <= 0 {
		return block, nil, nil
	}

	n, err := countToolResultTokens(m, block.Content)
	if err != nil {
		return nil, nil, err
	}
	if n <= limit {
		return block, nil, nil
	}

	blocks := cloneContentBlocks(block.Content)
	if len(blocks) == 0 {
		return block, nil, nil
	}

	boundary := 0
	for i := len(blocks) - 1; i >= 0; i-- {
		prefix := cloneContentBlocks(blocks[:i])
		cur, err := countToolResultTokens(m, prefix)
		if err != nil {
			return nil, nil, err
		}
		if cur < limit {
			boundary = i
			break
		}
	}

	reserved := cloneContentBlocks(blocks[:boundary])
	offload := cloneContentBlocks(blocks[boundary+1:])

	if boundary < len(blocks) {
		boundaryBlock, ok := blocks[boundary].(*message.TextBlock)
		if ok && boundaryBlock != nil {
			reservedText, offloadText := truncateTextBlockByTokenBudget(m, reserved, boundaryBlock, limit)
			if reservedText != "" {
				appendTextBlock(&reserved, reservedText, boundaryBlock)
			}
			if offloadText != "" {
				prependTextBlock(&offload, offloadText, boundaryBlock)
			}
		} else if len(offload) == 0 {
			offload = []message.ContentBlock{blocks[boundary]}
		} else {
			offload = append([]message.ContentBlock{blocks[boundary]}, offload...)
		}
	}

	if len(offload) == 0 {
		return block, nil, nil
	}

	return newToolResultBlock(block, reserved), newToolResultBlock(block, offload), nil
}

func (a *ReActAgent) compressToolResultBlocks(
	ctx context.Context,
	toolCallID string,
	blocks []message.ContentBlock,
	isError bool,
) []message.ContentBlock {
	limit := a.contextConfig.ToolResultLimit
	if limit <= 0 {
		limit = 3000
	}
	src := message.NewToolResultBlock(toolCallID, blocks, isError)
	reserved, offload, err := SplitToolResultForCompression(a.chatModel, src, limit)
	if err != nil || offload == nil {
		if reserved != nil {
			return reserved.Content
		}
		return blocks
	}

	var reminder string
	if a.offloader != nil {
		data, _ := json.Marshal(offload.Content)
		if path, err := a.offloader.OffloadToolResult(ctx, a.sessionID(), toolCallID, data); err == nil && path != "" {
			reminder = fmt.Sprintf(toolResultTruncationReminder,
				fmt.Sprintf(" You can refer to the file in '%s' for the truncated content if needed.", path))
		} else {
			reminder = fmt.Sprintf(toolResultTruncationReminder, "")
		}
	} else {
		reminder = fmt.Sprintf(toolResultTruncationReminder, "")
	}

	out := append([]message.ContentBlock(nil), reserved.Content...)
	appendTextBlock(&out, reminder, nil)
	return out
}

func (a *ReActAgent) sessionID() string {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState != nil && a.runtimeState.AgentID != "" {
		return a.runtimeState.AgentID
	}
	if a.Base != nil && a.Base.ID != "" {
		return a.Base.ID
	}
	return a.Base.Name
}

func countToolResultTokens(m model.ChatModel, blocks []message.ContentBlock) (int, error) {
	msg := message.NewMsg().Role(message.RoleAssistant).Content(blocks...).Build()
	return model.CountTokens(m, []*message.Msg{msg}, nil)
}

func truncateTextBlockByTokenBudget(
	m model.ChatModel,
	reserved []message.ContentBlock,
	boundary *message.TextBlock,
	limit int,
) (reservedText, offloadText string) {
	text := boundary.Text
	if text == "" {
		return "", ""
	}
	cur, err := countToolResultTokens(m, reserved)
	if err != nil {
		return text, ""
	}
	curPlus, err := countToolResultTokens(m, append(reserved, message.NewTextBlock(text)))
	if err != nil {
		return text, ""
	}
	tokenDelta := curPlus - cur
	remaining := limit - cur
	if remaining <= 0 {
		return "", text
	}
	if tokenDelta <= 0 {
		if remaining > 0 {
			return text, ""
		}
		return "", text
	}
	reservedChars := int(float64(remaining) / float64(tokenDelta) * float64(len(text)))
	if reservedChars < 0 {
		reservedChars = 0
	}
	if reservedChars > len(text) {
		reservedChars = len(text)
	}
	return text[:reservedChars], text[reservedChars:]
}

func cloneContentBlocks(blocks []message.ContentBlock) []message.ContentBlock {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]message.ContentBlock, len(blocks))
	copy(out, blocks)
	return out
}

func newToolResultBlock(src *message.ToolResultBlock, content []message.ContentBlock) *message.ToolResultBlock {
	return message.NewToolResultBlock(src.ToolUseID, cloneContentBlocks(content), src.IsError)
}

func appendTextBlock(blocks *[]message.ContentBlock, text string, _ *message.TextBlock) {
	if text == "" {
		return
	}
	if len(*blocks) > 0 {
		if tb, ok := (*blocks)[len(*blocks)-1].(*message.TextBlock); ok {
			tb.Text += text
			return
		}
	}
	*blocks = append(*blocks, message.NewTextBlock(text))
}

func prependTextBlock(blocks *[]message.ContentBlock, text string, _ *message.TextBlock) {
	if text == "" {
		return
	}
	if len(*blocks) > 0 {
		if tb, ok := (*blocks)[0].(*message.TextBlock); ok {
			tb.Text = text + tb.Text
			return
		}
	}
	*blocks = append([]message.ContentBlock{message.NewTextBlock(text)}, *blocks...)
}

func blocksTextSummary(blocks []message.ContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if tb, ok := b.(*message.TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	return sb.String()
}
