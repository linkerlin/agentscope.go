package memory

import (
	"sync"

	"github.com/linkerlin/agentscope.go/message"
)

// Tokenizer 用于按 token 预算裁剪对话窗口（可为近似实现）
type Tokenizer interface {
	Count(msg *message.Msg) int
	CountText(text string) int
}

// RuneTokenizer 以 Unicode 码点数量近似 token 数（零依赖，适合作为默认）
type RuneTokenizer struct{}

// Count 统计消息的大致 token 数（基于文本块拼接）
func (RuneTokenizer) Count(msg *message.Msg) int {
	if msg == nil {
		return 0
	}
	return len([]rune(msg.GetTextContent()))
}

// CountText 统计文本码点数
func (RuneTokenizer) CountText(text string) int {
	return len([]rune(text))
}

// WindowMemory 在内存中维护滑动窗口，可按条数与（可选）token 上限裁剪
type WindowMemory struct {
	mu          sync.RWMutex
	msgs        []*message.Msg
	maxMessages int
	maxTokens   int
	tokenizer   Tokenizer
}

// WindowOptions 配置窗口内存
type WindowOptions struct {
	MaxMessages int
	MaxTokens   int
	Tokenizer   Tokenizer
}

// NewWindowMemory 创建窗口内存；maxMessages、maxTokens 均 <=0 时表示不限制该项
func NewWindowMemory(opts WindowOptions) *WindowMemory {
	tok := opts.Tokenizer
	if tok == nil && opts.MaxTokens > 0 {
		tok = RuneTokenizer{}
	}
	return &WindowMemory{
		msgs:        make([]*message.Msg, 0),
		maxMessages: opts.MaxMessages,
		maxTokens:   opts.MaxTokens,
		tokenizer:   tok,
	}
}

func (m *WindowMemory) Add(msg *message.Msg) error {
	if msg == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	m.trimLocked()
	return nil
}

func (m *WindowMemory) trimLocked() {
	if m.maxMessages > 0 {
		for len(m.msgs) > m.maxMessages {
			m.msgs = m.msgs[1:]
		}
	}
	if m.maxTokens > 0 && m.tokenizer != nil {
		m.trimByTokensLocked()
	}
}

func (m *WindowMemory) trimByTokensLocked() {
	total := 0
	for i := len(m.msgs) - 1; i >= 0; i-- {
		total += m.tokenizer.Count(m.msgs[i])
		if total > m.maxTokens {
			m.msgs = m.msgs[i+1:]
			return
		}
	}
}

func (m *WindowMemory) GetAll() ([]*message.Msg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*message.Msg, len(m.msgs))
	copy(out, m.msgs)
	return out, nil
}

func (m *WindowMemory) GetRecent(n int) ([]*message.Msg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n >= len(m.msgs) {
		out := make([]*message.Msg, len(m.msgs))
		copy(out, m.msgs)
		return out, nil
	}
	out := make([]*message.Msg, n)
	copy(out, m.msgs[len(m.msgs)-n:])
	return out, nil
}

func (m *WindowMemory) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = nil
	return nil
}

func (m *WindowMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.msgs)
}
