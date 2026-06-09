package formatter

import (
	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
)

const defaultConversationHistoryPrompt = "# Conversation History\n" +
	"The content between <history></history> tags contains your conversation history\n"

// MessageGroup is a contiguous group of messages for multi-agent formatting.
type MessageGroup struct {
	Type string // "tool_sequence" | "agent_message"
	Msgs []*message.Msg
}

// GroupMessages splits messages into tool sequences and agent message groups.
func GroupMessages(msgs []*message.Msg) []MessageGroup {
	var groups []MessageGroup
	var groupType string
	var group []*message.Msg

	flush := func() {
		if groupType == "" || len(group) == 0 {
			return
		}
		groups = append(groups, MessageGroup{Type: groupType, Msgs: append([]*message.Msg(nil), group...)})
		group = nil
	}

	for _, msg := range msgs {
		isTool := len(msg.GetToolUseCalls()) > 0 || len(msg.GetToolResults()) > 0
		if groupType == "" {
			if isTool {
				groupType = "tool_sequence"
			} else {
				groupType = "agent_message"
			}
			group = append(group, msg)
			continue
		}
		if groupType == "tool_sequence" {
			if isTool {
				group = append(group, msg)
			} else {
				flush()
				groupType = "agent_message"
				group = append(group, msg)
			}
			continue
		}
		if isTool {
			flush()
			groupType = "tool_sequence"
			group = append(group, msg)
		} else {
			group = append(group, msg)
		}
	}
	flush()
	return groups
}

// FormatOpenAIMultiAgentMessages formats multi-agent conversations for OpenAI-compatible APIs.
func FormatOpenAIMultiAgentMessages(f *OpenAIFormatter, msgs []*message.Msg) []goopenai.ChatCompletionMessage {
	if f == nil {
		f = NewOpenAIFormatter()
	}
	if len(msgs) == 0 {
		return nil
	}

	start := 0
	var out []goopenai.ChatCompletionMessage
	if msgs[0].Role == message.RoleSystem {
		out = append(out, goopenai.ChatCompletionMessage{
			Role:    goopenai.ChatMessageRoleSystem,
			Content: msgs[0].GetTextContent(),
		})
		start = 1
	}

	firstAgent := true
	for _, g := range GroupMessages(msgs[start:]) {
		switch g.Type {
		case "tool_sequence":
			out = append(out, f.FormatMessagesTyped(g.Msgs)...)
		case "agent_message":
			out = append(out, formatOpenAIAgentMessageGroup(g.Msgs, firstAgent)...)
			firstAgent = false
		}
	}
	return out
}

func formatOpenAIAgentMessageGroup(msgs []*message.Msg, isFirst bool) []goopenai.ChatCompletionMessage {
	prompt := ""
	if isFirst {
		prompt = defaultConversationHistoryPrompt
	}

	var lines []string
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		name := msg.Name
		if name == "" {
			name = string(msg.Role)
		}
		text := msg.GetTextContent()
		if text != "" {
			lines = append(lines, name+": "+text)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	body := prompt + "<history>\n" + joinLines(lines) + "\n</history>"
	return []goopenai.ChatCompletionMessage{{
		Role:    goopenai.ChatMessageRoleUser,
		Content: body,
	}}
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}

// MultiAgentOpenAIFormatter formats multi-agent conversations for OpenAI-compatible APIs.
type MultiAgentOpenAIFormatter struct {
	*OpenAIFormatter
}

func NewMultiAgentOpenAIFormatter() *MultiAgentOpenAIFormatter {
	return &MultiAgentOpenAIFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

func (f *MultiAgentOpenAIFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	return FormatOpenAIMultiAgentMessages(f.OpenAIFormatter, msgs), nil
}

func (f *MultiAgentOpenAIFormatter) FormatMessagesTyped(msgs []*message.Msg) []goopenai.ChatCompletionMessage {
	return FormatOpenAIMultiAgentMessages(f.OpenAIFormatter, msgs)
}
