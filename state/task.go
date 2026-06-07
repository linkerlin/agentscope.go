package state

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskState represents the lifecycle state of an agent-managed task.
type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskInProgress TaskState = "in_progress"
	TaskCompleted  TaskState = "completed"
)

// AgentTask is a structured task tracked by the agent at runtime.
type AgentTask struct {
	ID          string         `json:"id"`
	Subject     string         `json:"subject"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	State       TaskState      `json:"state"`
	Owner       string         `json:"owner,omitempty"`
	Blocks      []string       `json:"blocks,omitempty"`
	BlockedBy   []string       `json:"blocked_by,omitempty"`
}

// TaskStore manages agent tasks in memory.
type TaskStore struct {
	mu    sync.RWMutex
	tasks []AgentTask
}

// NewTaskStore creates an empty task store.
func NewTaskStore() *TaskStore {
	return &TaskStore{}
}

// Create adds a new pending task.
func (s *TaskStore) Create(subject, description string, metadata map[string]any) *AgentTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := metadata
	if meta == nil {
		meta = map[string]any{}
	}
	task := AgentTask{
		ID:          uuid.New().String(),
		Subject:     subject,
		Description: description,
		Metadata:    meta,
		CreatedAt:   time.Now(),
		State:       TaskPending,
	}
	s.tasks = append(s.tasks, task)
	return &task
}

// Get returns a task by ID.
func (s *TaskStore) Get(id string) (*AgentTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			t := s.tasks[i]
			return &t, true
		}
	}
	return nil, false
}

// List returns a copy of all tasks.
func (s *TaskStore) List() []AgentTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AgentTask, len(s.tasks))
	copy(out, s.tasks)
	return out
}

// Update applies partial updates to a task. fn returns true to delete the task.
func (s *TaskStore) Update(id string, fn func(*AgentTask) bool) (*AgentTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].ID != id {
			continue
		}
		if fn(&s.tasks[i]) {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return nil, true
		}
		t := s.tasks[i]
		return &t, true
	}
	return nil, false
}

// AddBlockRelation records that blockID blocks blockedByID.
func (s *TaskStore) AddBlockRelation(blockID, blockedByID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make(map[string]bool, len(s.tasks))
	for _, t := range s.tasks {
		ids[t.ID] = true
	}
	if !ids[blockID] || !ids[blockedByID] {
		return
	}
	for i := range s.tasks {
		if s.tasks[i].ID == blockID {
			if !containsString(s.tasks[i].Blocks, blockedByID) {
				s.tasks[i].Blocks = append(s.tasks[i].Blocks, blockedByID)
			}
		}
		if s.tasks[i].ID == blockedByID {
			if !containsString(s.tasks[i].BlockedBy, blockID) {
				s.tasks[i].BlockedBy = append(s.tasks[i].BlockedBy, blockID)
			}
		}
	}
}

func containsString(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
