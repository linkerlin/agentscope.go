package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestWindowMemoryMaxMessages(t *testing.T) {
	m := NewWindowMemory(WindowOptions{MaxMessages: 2})
	_ = m.Add(msg("a"))
	_ = m.Add(msg("b"))
	_ = m.Add(msg("c"))
	all, _ := m.GetAll()
	if len(all) != 2 {
		t.Fatalf("len=%d", len(all))
	}
	if all[0].GetTextContent() != "b" || all[1].GetTextContent() != "c" {
		t.Fatalf("content %#v", []string{all[0].GetTextContent(), all[1].GetTextContent()})
	}
}

func TestWindowMemoryMaxTokens(t *testing.T) {
	m := NewWindowMemory(WindowOptions{
		MaxTokens: 5,
		Tokenizer: RuneTokenizer{},
	})
	_ = m.Add(msg("abcde")) // 5 runes
	_ = m.Add(msg("f"))     // should drop first
	all, _ := m.GetAll()
	if len(all) != 1 || all[0].GetTextContent() != "f" {
		t.Fatalf("got %#v", all[0].GetTextContent())
	}
}

func msg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
}
