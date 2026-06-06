package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStorage is an in-memory implementation of Storage for development and testing.
type MemoryStorage struct {
	mu          sync.RWMutex
	users       map[string]*User
	sessions    map[string]*Session
	agents      map[string]*AgentConfig
	credentials map[string]*Credential
	messages    map[string][]*StoredMessage // sessionID -> messages
	snapshots   map[string]*AgentSnapshot
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		users:       make(map[string]*User),
		sessions:    make(map[string]*Session),
		agents:      make(map[string]*AgentConfig),
		credentials: make(map[string]*Credential),
		messages:    make(map[string][]*StoredMessage),
		snapshots:   make(map[string]*AgentSnapshot),
	}
}

func (s *MemoryStorage) SaveUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user.UpdatedAt = time.Now()
	s.users[user.ID] = user
	return nil
}

func (s *MemoryStorage) GetUser(ctx context.Context, id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	return u, nil
}

func (s *MemoryStorage) DeleteUser(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, id)
	return nil
}

func (s *MemoryStorage) SaveSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.UpdatedAt = time.Now()
	s.sessions[session.ID] = session
	return nil
}

func (s *MemoryStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	se, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return se, nil
}

func (s *MemoryStorage) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Session
	for _, se := range s.sessions {
		if se.UserID == userID {
			out = append(out, se)
		}
	}
	return out, nil
}

func (s *MemoryStorage) DeleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	delete(s.messages, id)
	delete(s.snapshots, id)
	return nil
}

func (s *MemoryStorage) SaveAgentConfig(ctx context.Context, cfg *AgentConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.UpdatedAt = time.Now()
	s.agents[cfg.ID] = cfg
	return nil
}

func (s *MemoryStorage) GetAgentConfig(ctx context.Context, id string) (*AgentConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent config not found: %s", id)
	}
	return c, nil
}

func (s *MemoryStorage) ListAgentConfigsByUser(ctx context.Context, userID string) ([]*AgentConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*AgentConfig
	for _, c := range s.agents {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *MemoryStorage) DeleteAgentConfig(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, id)
	return nil
}

func (s *MemoryStorage) SaveCredential(ctx context.Context, cred *Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cred.UpdatedAt = time.Now()
	s.credentials[cred.ID] = cred
	return nil
}

func (s *MemoryStorage) GetCredential(ctx context.Context, id string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.credentials[id]
	if !ok {
		return nil, fmt.Errorf("credential not found: %s", id)
	}
	return c, nil
}

func (s *MemoryStorage) ListCredentialsByUser(ctx context.Context, userID string) ([]*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Credential
	for _, c := range s.credentials {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *MemoryStorage) DeleteCredential(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentials, id)
	return nil
}

func (s *MemoryStorage) SaveMessage(ctx context.Context, msg *StoredMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[msg.SessionID] = append(s.messages[msg.SessionID], msg)
	return nil
}

func (s *MemoryStorage) ListMessagesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.messages[sessionID]
	if offset >= len(msgs) {
		return nil, nil
	}
	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	out := make([]*StoredMessage, end-offset)
	copy(out, msgs[offset:end])
	return out, nil
}

func (s *MemoryStorage) DeleteMessagesBySession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, sessionID)
	return nil
}

func (s *MemoryStorage) SaveSnapshot(ctx context.Context, snap *AgentSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snap.SessionID] = snap
	return nil
}

func (s *MemoryStorage) GetSnapshot(ctx context.Context, sessionID string) (*AgentSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[sessionID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", sessionID)
	}
	return snap, nil
}

func (s *MemoryStorage) DeleteSnapshot(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, sessionID)
	return nil
}

// Compile-time check.
var _ Storage = (*MemoryStorage)(nil)
