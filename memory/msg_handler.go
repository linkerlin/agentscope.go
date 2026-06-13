package memory

import (
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
)

// MarkStore 按消息 ID 维护标记（线程安全由外层 ReMeFileMemory 的 mu 保证）
type MarkStore struct {
	marks map[string][]MessageMark
}

// NewMarkStore 创建空标记表
func NewMarkStore() *MarkStore {
	return &MarkStore{marks: make(map[string][]MessageMark)}
}

// Get 返回某条消息的标记副本
func (s *MarkStore) Get(msgID string) []MessageMark {
	if s == nil || s.marks == nil {
		return nil
	}
	m := s.marks[msgID]
	out := make([]MessageMark, len(m))
	copy(out, m)
	return out
}

// Add 追加标记（去重）
func (s *MarkStore) Add(msgID string, mark MessageMark) {
	if s.marks == nil {
		s.marks = make(map[string][]MessageMark)
	}
	for _, existing := range s.marks[msgID] {
		if existing == mark {
			return
		}
	}
	s.marks[msgID] = append(s.marks[msgID], mark)
}

// Has 是否包含某标记
func (s *MarkStore) Has(msgID string, mark MessageMark) bool {
	for _, m := range s.Get(msgID) {
		if m == mark {
			return true
		}
	}
	return false
}

// Clear 清空某条消息标记
func (s *MarkStore) Clear(msgID string) {
	if s.marks != nil {
		delete(s.marks, msgID)
	}
}

// ToMap 导出为可 JSON 序列化的 map
func (s *MarkStore) ToMap() map[string][]string {
	if s == nil || s.marks == nil {
		return nil
	}
	out := make(map[string][]string)
	for k, v := range s.marks {
		ss := make([]string, len(v))
		for i, m := range v {
			ss[i] = string(m)
		}
		out[k] = ss
	}
	return out
}

// LoadMarkStore 从 map 恢复
func LoadMarkStore(m map[string][]string) *MarkStore {
	s := NewMarkStore()
	for id, list := range m {
		for _, x := range list {
			s.Add(id, MessageMark(x))
		}
	}
	return s
}

// FormatMessagesPlain 将消息格式化为可读文本（供压缩器等使用）
func FormatMessagesPlain(msgs []*message.Msg) string {
	var parts []string
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		role := string(msg.Role)
		content := msg.GetTextContent()
		if content == "" && len(msg.GetToolUseCalls()) > 0 {
			for _, tc := range msg.GetToolUseCalls() {
				content += fmt.Sprintf("[tool_use %s %s]", tc.Name, tc.ID)
			}
		}
		parts = append(parts, fmt.Sprintf("[%s]: %s", role, content))
	}
	return strings.Join(parts, "\n\n")
}
