package memory

import (
	"github.com/linkerlin/agentscope.go/message"
)

// TokenCounter 对文本与消息做 token 估算（可替换为更精确实现）
type TokenCounter interface {
	Count(text string) (int, error)
	CountMessages(msgs []*message.Msg) (int, error)
}

// SimpleTokenCounter 按字符粗略换算 token（约 4 字符/token）
type SimpleTokenCounter struct {
	CharsPerToken int
}

// NewSimpleTokenCounter 默认 CharsPerToken=4
func NewSimpleTokenCounter() *SimpleTokenCounter {
	return &SimpleTokenCounter{CharsPerToken: 4}
}

// Count 估算文本 token 数
func (c *SimpleTokenCounter) Count(text string) (int, error) {
	n := len([]rune(text))
	cp := c.CharsPerToken
	if cp < 1 {
		cp = 4
	}
	return n / cp, nil
}

// CountMessages 估算多条消息总 token
func (c *SimpleTokenCounter) CountMessages(msgs []*message.Msg) (int, error) {
	var total int
	for _, m := range msgs {
		tokens, err := c.Count(m.GetTextContent())
		if err != nil {
			return 0, err
		}
		total += tokens
	}
	return total, nil
}
