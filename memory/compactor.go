package memory

import (
	"context"
	"errors"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

const compactorSystemPrompt = `You are a conversation summarizer. Output a structured summary with EXACTLY these sections as markdown headers:
## Goal
## Constraints
## Progress
## Key Decisions
## Next Steps
## Critical Context

Rules:
1. Be concise but preserve all critical information
2. Maintain continuity for the next conversation turn
3. Use bullet points where appropriate
4. If previous summary is provided, merge new information intelligently
5. Output ONLY the formatted summary, no extra text`

// Compactor 使用 ChatModel 单轮调用生成结构化摘要（不依赖 react 包）
type Compactor struct {
	model model.ChatModel
}

// NewCompactor 创建压缩器
func NewCompactor(m model.ChatModel) *Compactor {
	return &Compactor{model: m}
}

// Compact 将消息压缩为 CompactSummary
func (c *Compactor) Compact(ctx context.Context, msgs []*message.Msg, opts CompactOptions) (*CompactSummary, error) {
	if c == nil || c.model == nil {
		return nil, ErrCompactorNoModel
	}
	formatted := FormatMessagesPlain(msgs)
	var user strings.Builder
	user.WriteString("# Conversation\n")
	user.WriteString(formatted)
	user.WriteString("\n\n")
	if opts.PreviousSummary != "" {
		user.WriteString("# Previous Summary\n")
		user.WriteString(opts.PreviousSummary)
		user.WriteString("\n\nUpdate the summary with new information from the conversation.\n")
	} else {
		user.WriteString("Create a summary of this conversation.\n")
	}
	if opts.Language == "zh" {
		user.WriteString("\n请用中文输出。\n")
	}
	ms := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(compactorSystemPrompt).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(user.String()).Build(),
	}
	resp, err := c.model.Chat(ctx, ms)
	if err != nil {
		return nil, err
	}
	raw := resp.GetTextContent()
	s := parseCompactSummary(raw)
	s.Raw = raw
	return s, nil
}

// ErrCompactorNoModel 未配置模型
var ErrCompactorNoModel = errors.New("memory: compactor requires ChatModel")

func parseCompactSummary(raw string) *CompactSummary {
	s := &CompactSummary{}
	lines := strings.Split(raw, "\n")
	var section string
	var buf []string
	flush := func() {
		content := strings.TrimSpace(strings.Join(buf, "\n"))
		switch section {
		case "Goal":
			s.Goal = content
		case "Constraints":
			s.Constraints = splitLines(content)
		case "Progress":
			s.Progress = content
		case "Key Decisions":
			s.KeyDecisions = splitLines(content)
		case "Next Steps":
			s.NextSteps = splitLines(content)
		case "Critical Context":
			s.CriticalContext = splitLines(content)
		}
		buf = nil
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			flush()
			section = strings.TrimPrefix(line, "## ")
			section = strings.TrimSpace(section)
			continue
		}
		if section != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return s
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
