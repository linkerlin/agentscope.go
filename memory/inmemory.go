package memory

import (
	"sync"

	"github.com/linkerlin/agentscope.go/message"
)

// InMemoryMemory is a thread-safe, in-process Memory implementation
type InMemoryMemory struct {
	mu   sync.RWMutex
	msgs []*message.Msg
}

// NewInMemoryMemory creates a new InMemoryMemory
func NewInMemoryMemory() *InMemoryMemory {
	return &InMemoryMemory{}
}

func (m *InMemoryMemory) Add(msg *message.Msg) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return nil
}

func (m *InMemoryMemory) GetAll() ([]*message.Msg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*message.Msg, len(m.msgs))
	copy(result, m.msgs)
	return result, nil
}

func (m *InMemoryMemory) GetRecent(n int) ([]*message.Msg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n >= len(m.msgs) {
		result := make([]*message.Msg, len(m.msgs))
		copy(result, m.msgs)
		return result, nil
	}
	result := make([]*message.Msg, n)
	copy(result, m.msgs[len(m.msgs)-n:])
	return result, nil
}

func (m *InMemoryMemory) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = nil
	return nil
}

func (m *InMemoryMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.msgs)
}
