package session

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkerlin/agentscope.go/message"
)

// Session represents a conversation session
type Session struct {
	ID        string
	AgentName string
	Messages  []*message.Msg
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
	mu        sync.RWMutex
}

// SessionService manages sessions
type SessionService interface {
	Create(agentName string) (*Session, error)
	Get(sessionID string) (*Session, error)
	AddMessage(sessionID string, msg *message.Msg) error
	Delete(sessionID string) error
	List() ([]*Session, error)
}

// InMemorySessionService is an in-memory session service
type InMemorySessionService struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewInMemorySessionService() *InMemorySessionService {
	return &InMemorySessionService{sessions: make(map[string]*Session)}
}

func (s *InMemorySessionService) Create(agentName string) (*Session, error) {
	sess := &Session{
		ID:        uuid.New().String(),
		AgentName: agentName,
		Metadata:  make(map[string]any),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
	return sess, nil
}

func (s *InMemorySessionService) Get(sessionID string) (*Session, error) {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.New("session not found: " + sessionID)
	}
	return sess, nil
}

func (s *InMemorySessionService) AddMessage(sessionID string, msg *message.Msg) error {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return errors.New("session not found: " + sessionID)
	}
	sess.mu.Lock()
	sess.Messages = append(sess.Messages, msg)
	sess.UpdatedAt = time.Now()
	sess.mu.Unlock()
	return nil
}

func (s *InMemorySessionService) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		return errors.New("session not found: " + sessionID)
	}
	delete(s.sessions, sessionID)
	return nil
}

func (s *InMemorySessionService) List() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, sess)
	}
	return result, nil
}
