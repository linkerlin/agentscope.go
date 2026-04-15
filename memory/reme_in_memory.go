package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// ReMeInMemoryMemory 纯内存消息缓存与标记管理（支持 dialog 文件追加）
type ReMeInMemoryMemory struct {
	mu       sync.RWMutex
	msgs     []*message.Msg
	marks    *MarkStore
	compSum  string
	longTerm string

	dialogPath string
}

// NewReMeInMemoryMemory 创建内存记忆实例
func NewReMeInMemoryMemory(dialogPath string) *ReMeInMemoryMemory {
	return &ReMeInMemoryMemory{
		marks:      NewMarkStore(),
		dialogPath: dialogPath,
	}
}

// Add 追加消息并异步写入 dialog 文件
func (m *ReMeInMemoryMemory) Add(msg *message.Msg) error {
	if msg == nil {
		return nil
	}
	m.mu.Lock()
	m.msgs = append(m.msgs, msg)
	m.mu.Unlock()
	return m.appendToDialog([]*message.Msg{msg})
}

// GetAll 返回未删除视图（可选排除 compressed）
func (m *ReMeInMemoryMemory) GetAll() ([]*message.Msg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getFiltered(true), nil
}

// GetRecent 最近 n 条（过滤后）
func (m *ReMeInMemoryMemory) GetRecent(n int) ([]*message.Msg, error) {
	all, err := m.GetAll()
	if err != nil {
		return nil, err
	}
	if n >= len(all) {
		return append([]*message.Msg(nil), all...), nil
	}
	return append([]*message.Msg(nil), all[len(all)-n:]...), nil
}

// Clear 清空内存状态
func (m *ReMeInMemoryMemory) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = nil
	m.marks = NewMarkStore()
	m.compSum = ""
	m.longTerm = ""
	return nil
}

// Size 条数
func (m *ReMeInMemoryMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.getFiltered(false))
}

func (m *ReMeInMemoryMemory) getFiltered(excludeCompressed bool) []*message.Msg {
	var out []*message.Msg
	for _, msg := range m.msgs {
		if msg == nil {
			continue
		}
		if m.marks.Has(msg.ID, MarkDeleted) {
			continue
		}
		if excludeCompressed && m.marks.Has(msg.ID, MarkCompressed) {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// SetLongTermMemory 设置长期记忆文本
func (m *ReMeInMemoryMemory) SetLongTermMemory(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.longTerm = text
}

// GetLongTermMemory 读取长期记忆文本
func (m *ReMeInMemoryMemory) GetLongTermMemory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.longTerm
}

// SetCompSum 设置压缩摘要
func (m *ReMeInMemoryMemory) SetCompSum(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compSum = text
}

// GetCompSum 读取压缩摘要
func (m *ReMeInMemoryMemory) GetCompSum() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.compSum
}

// Marks 返回标记存储
func (m *ReMeInMemoryMemory) Marks() *MarkStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.marks
}

// SetMarks 替换标记存储
func (m *ReMeInMemoryMemory) SetMarks(marks *MarkStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.marks = marks
}

// Msgs 返回内部消息切片副本（原始引用）
func (m *ReMeInMemoryMemory) Msgs() []*message.Msg {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneMsgSlice(m.msgs)
}

// Snapshot 返回可序列化的内存状态快照
func (m *ReMeInMemoryMemory) Snapshot() *InMemorySnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &InMemorySnapshot{
		CompressedSummary: m.compSum,
		LongTermMemory:    m.longTerm,
		Marks:             m.marks.ToMap(),
	}
}

// Restore 从快照恢复内存状态
func (m *ReMeInMemoryMemory) Restore(snap *InMemorySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compSum = snap.CompressedSummary
	m.longTerm = snap.LongTermMemory
	m.marks = LoadMarkStore(snap.Marks)
}

// InMemorySnapshot 内存状态快照
type InMemorySnapshot struct {
	CompressedSummary string              `json:"compressed_summary"`
	LongTermMemory    string              `json:"long_term_memory"`
	Marks             map[string][]string `json:"marks"`
}

func (m *ReMeInMemoryMemory) appendToDialog(msgs []*message.Msg) error {
	if len(msgs) == 0 || m.dialogPath == "" {
		return nil
	}
	dateStr := time.Now().Format("2006-01-02")
	filename := filepath.Join(m.dialogPath, dateStr+".jsonl")
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, msg := range msgs {
		data, _ := json.Marshal(msg)
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}
