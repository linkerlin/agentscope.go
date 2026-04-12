package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// Summarizer 将对话摘要写入 workingDir/memory/YYYY-MM-DD.md
type Summarizer struct {
	Model      model.ChatModel
	WorkingDir string
}

// SummarizeToDailyFile 把消息摘要追加到当日 markdown 文件
func (s *Summarizer) SummarizeToDailyFile(ctx context.Context, msgs []*message.Msg) error {
	if s == nil || s.Model == nil {
		return ErrCompactorNoModel
	}
	dir := filepath.Join(s.WorkingDir, "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	day := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, day+".md")
	formatted := FormatMessagesPlain(msgs)
	prompt := "Summarize the following for long-term memory as bullet points under ## Notes:\n\n" + formatted
	out, err := s.Model.Chat(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
	})
	if err != nil {
		return err
	}
	text := out.GetTextContent()
	var sb strings.Builder
	sb.WriteString("# Daily summary ")
	sb.WriteString(day)
	sb.WriteString("\n\n")
	sb.WriteString(text)
	sb.WriteString("\n")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(sb.String())
	return err
}

// AppendToMemoryMD 将一段 Markdown 追加到 workingDir/memory/MEMORY.md（演进方案中的长期记忆汇总文件）
func (s *Summarizer) AppendToMemoryMD(title, body string) error {
	if s == nil || s.WorkingDir == "" {
		return errors.New("memory: summarizer requires WorkingDir")
	}
	dir := filepath.Join(s.WorkingDir, "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "MEMORY.md")
	var sb strings.Builder
	if title != "" {
		sb.WriteString("## ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(sb.String())
	return err
}
