package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linkerlin/agentscope.go/message"
)

// ToolResultCompactor 将过长工具结果落盘并替换为短引用
type ToolResultCompactor struct {
	baseDir       string
	recentMax     int
	oldMax        int
	retentionDays int
}

// NewToolResultCompactor baseDir 一般为 workingPath/tool_result
func NewToolResultCompactor(baseDir string, recentMaxBytes, oldMaxBytes, retentionDays int) *ToolResultCompactor {
	if recentMaxBytes <= 0 {
		recentMaxBytes = 100 * 1024
	}
	if oldMaxBytes <= 0 {
		oldMaxBytes = 3000
	}
	if retentionDays <= 0 {
		retentionDays = 3
	}
	return &ToolResultCompactor{
		baseDir:       baseDir,
		recentMax:     recentMaxBytes,
		oldMax:        oldMaxBytes,
		retentionDays: retentionDays,
	}
}

// Compact 处理消息列表：recentTail 表示尾部若干条视为「最近」用较宽松阈值
func (c *ToolResultCompactor) Compact(msgs []*message.Msg, recentTail int) ([]*message.Msg, error) {
	if c == nil {
		return msgs, nil
	}
	_ = os.MkdirAll(c.baseDir, 0o755)
	out := make([]*message.Msg, len(msgs))
	for i := range msgs {
		m := msgs[i]
		if m == nil {
			out[i] = nil
			continue
		}
		isRecent := i >= len(msgs)-recentTail
		nm, err := c.compactOne(m, isRecent)
		if err != nil {
			return nil, err
		}
		out[i] = nm
	}
	return out, nil
}

func (c *ToolResultCompactor) compactOne(m *message.Msg, isRecent bool) (*message.Msg, error) {
	maxB := c.oldMax
	if isRecent {
		maxB = c.recentMax
	}
	b := message.NewMsg().Role(m.Role).Name(m.Name)
	if len(m.Metadata) > 0 {
		ks := make([]string, 0, len(m.Metadata))
		for k := range m.Metadata {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			b.Metadata(k, m.Metadata[k])
		}
	}
	for _, block := range m.Content {
		switch t := block.(type) {
		case *message.TextBlock:
			b.Content(message.NewTextBlock(t.Text))
		case *message.ToolResultBlock:
			text := contentBytes(t.Content)
			if text <= maxB {
				b.Content(message.NewToolResultBlock(t.ToolUseID, t.Content, t.IsError))
				continue
			}
			path, err := c.writeOverflow(t.ToolUseID, textFromBlocks(t.Content))
			if err != nil {
				return nil, err
			}
			short := fmt.Sprintf("[tool output truncated, see file: %s]", path)
			b.Content(message.NewToolResultBlock(t.ToolUseID, []message.ContentBlock{
				message.NewTextBlock(short),
			}, t.IsError))
		default:
			b.Content(block)
		}
	}
	return b.Build(), nil
}

func contentBytes(blocks []message.ContentBlock) int {
	n := 0
	for _, b := range blocks {
		if tb, ok := b.(*message.TextBlock); ok {
			n += len(tb.Text)
		}
	}
	return n
}

func textFromBlocks(blocks []message.ContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if tb, ok := b.(*message.TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	return sb.String()
}

func (c *ToolResultCompactor) writeOverflow(toolUseID, full string) (string, error) {
	name := uuid.New().String() + ".txt"
	path := filepath.Join(c.baseDir, name)
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		return "", err
	}
	_ = toolUseID
	return path, nil
}

// PurgeExpired 删除早于 retention 的文件（惰性清理）
func (c *ToolResultCompactor) PurgeExpired() error {
	if c == nil {
		return nil
	}
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-time.Duration(c.retentionDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(c.baseDir, e.Name()))
		}
	}
	return nil
}
