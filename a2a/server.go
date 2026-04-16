package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AgentCard describes the capabilities of an A2A agent.
type AgentCard struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	URL         string            `json:"url"`
	Version     string            `json:"version"`
	Capabilities []string         `json:"capabilities,omitempty"`
	Skills      []SkillInfo       `json:"skills,omitempty"`
}

// SkillInfo describes a skill exposed by the agent.
type SkillInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Task represents an A2A task unit.
type Task struct {
	ID        string      `json:"id"`
	Status    TaskStatus  `json:"status"`
	Messages  []Message   `json:"messages"`
	Artifacts []Artifact  `json:"artifacts,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// TaskStatus indicates the lifecycle state of a task.
type TaskStatus string

const (
	TaskStatusSubmitted  TaskStatus = "submitted"
	TaskStatusWorking    TaskStatus = "working"
	TaskStatusInputRequired TaskStatus = "input-required"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCanceled   TaskStatus = "canceled"
)

// Artifact represents an output artifact produced by the agent.
type Artifact struct {
	Name    string         `json:"name,omitempty"`
	Parts   []Message      `json:"parts"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskUpdateRequest is the payload for sending a message to a task.
type TaskUpdateRequest struct {
	ID       string    `json:"id"`
	Message  *Message  `json:"message,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskStore persists and retrieves tasks.
type TaskStore interface {
	Get(taskID string) (*Task, error)
	Save(task *Task) error
}

// InMemoryTaskStore is a simple in-memory TaskStore.
type InMemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewInMemoryTaskStore creates a new in-memory task store.
func NewInMemoryTaskStore() *InMemoryTaskStore {
	return &InMemoryTaskStore{tasks: make(map[string]*Task)}
}

// Get retrieves a task by ID.
func (s *InMemoryTaskStore) Get(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	// Return a shallow copy to avoid external mutation racing with Save.
	cp := *t
	return &cp, nil
}

// Save stores a task.
func (s *InMemoryTaskStore) Save(task *Task) error {
	if task == nil {
		return errors.New("nil task")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	s.tasks[task.ID] = &cp
	return nil
}

// AgentRunner executes an agent call given an A2A message.
type AgentRunner interface {
	Run(ctx context.Context, msg *Message) (*Message, error)
}

// StreamingAgentRunner extends AgentRunner with streaming support.
type StreamingAgentRunner interface {
	AgentRunner
	RunStream(ctx context.Context, msg *Message) (<-chan *Message, error)
}

// Server is the minimal A2A HTTP server.
type Server struct {
	card    AgentCard
	runner  AgentRunner
	store   TaskStore
	mux     *http.ServeMux
}

// NewServer creates an A2A HTTP server.
func NewServer(card AgentCard, runner AgentRunner, store TaskStore) *Server {
	if store == nil {
		store = NewInMemoryTaskStore()
	}
	s := &Server{
		card:   card,
		runner: runner,
		store:  store,
		mux:    http.NewServeMux(),
	}
	s.mux.HandleFunc("/.well-known/agent.json", s.handleAgentCard)
	s.mux.HandleFunc("/task/send", s.handleTaskSend)
	s.mux.HandleFunc("/task/sendSubscribe", s.handleTaskSendSubscribe)
	s.mux.HandleFunc("/task/", s.handleTaskGet)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Addr returns the recommended listen address (host:port) derived from the card URL.
func (s *Server) Addr() string {
	u := s.card.URL
	if u == "" {
		return ":8080"
	}
	return u
}

func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.card)
}

func (s *Server) handleTaskSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req TaskUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	task, _ := s.store.Get(req.ID)
	if task == nil {
		task = &Task{
			ID:        req.ID,
			Status:    TaskStatusSubmitted,
			Messages:  []Message{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	if req.Message != nil {
		task.Messages = append(task.Messages, *req.Message)
	}
	task.Status = TaskStatusWorking
	task.UpdatedAt = time.Now()
	_ = s.store.Save(task)

	// Snapshot for the response so the async runner cannot mutate it.
	respTask := *task

	go func(taskID string, msgs []Message) {
		if s.runner == nil || req.Message == nil {
			return
		}
		t, _ := s.store.Get(taskID)
		if t == nil {
			return
		}
		resp, err := s.runner.Run(ctx, req.Message)
		if err != nil {
			t.Status = TaskStatusFailed
			t.Messages = append(msgs, Message{
				Role:    "agent",
				Content: fmt.Sprintf("error: %v", err),
			})
		} else if resp != nil {
			t.Status = TaskStatusCompleted
			t.Messages = append(msgs, *resp)
		} else {
			t.Status = TaskStatusCompleted
		}
		t.UpdatedAt = time.Now()
		_ = s.store.Save(t)
	}(task.ID, append([]Message(nil), task.Messages...))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(respTask)
}

func (s *Server) handleTaskSendSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req TaskUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	streamRunner, ok := s.runner.(StreamingAgentRunner)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusNotImplemented)
		return
	}

	ctx := r.Context()
	task, _ := s.store.Get(req.ID)
	if task == nil {
		task = &Task{
			ID:        req.ID,
			Status:    TaskStatusSubmitted,
			Messages:  []Message{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
	if req.Message != nil {
		task.Messages = append(task.Messages, *req.Message)
	}
	task.Status = TaskStatusWorking
	task.UpdatedAt = time.Now()
	_ = s.store.Save(task)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusAccepted)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	go func(taskID string, msgs []Message) {
		if req.Message == nil {
			return
		}
		t, _ := s.store.Get(taskID)
		if t == nil {
			return
		}
		ch, err := streamRunner.RunStream(ctx, req.Message)
		if err != nil {
			t.Status = TaskStatusFailed
			t.Messages = append(msgs, Message{Role: "agent", Content: fmt.Sprintf("error: %v", err)})
			t.UpdatedAt = time.Now()
			_ = s.store.Save(t)
			return
		}
		var finalMsg *Message
		for msg := range ch {
			if msg == nil {
				continue
			}
			finalMsg = msg
			t.Messages = append(msgs, *finalMsg)
			_ = s.store.Save(t)
		}
		if finalMsg != nil {
			t.Status = TaskStatusCompleted
		} else {
			t.Status = TaskStatusCompleted
		}
		t.UpdatedAt = time.Now()
		_ = s.store.Save(t)
	}(task.ID, append([]Message(nil), task.Messages...))

	// Send the initial task snapshot as the first SSE event.
	_ = json.NewEncoder(w).Encode(task)
	flusher.Flush()
}

func (s *Server) handleTaskGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Path[len("/task/"):]
	if taskID == "" {
		http.Error(w, "missing task id", http.StatusBadRequest)
		return
	}
	task, err := s.store.Get(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(task)
}

// NewTaskID generates a new task identifier.
func NewTaskID() string {
	return uuid.New().String()
}
