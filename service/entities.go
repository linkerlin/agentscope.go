package service

import (
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// User represents a tenant in the multi-tenant service.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	APIKey    string    `json:"api_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Session represents an agent conversation session.
type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	AgentID          string    `json:"agent_id"`
	Title            string    `json:"title,omitempty"`
	StateKey         string    `json:"state_key,omitempty"` // key to retrieve AgentState from StateStore
	SourceScheduleID string    `json:"source_schedule_id,omitempty"`
	WorkspaceID      string    `json:"workspace_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Schedule represents a persisted cron job for an agent.
type Schedule struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	AgentID         string    `json:"agent_id"`
	Name            string    `json:"name,omitempty"`
	Description     string    `json:"description,omitempty"`
	CronExpr        string    `json:"cron_expr"`
	Payload         string    `json:"payload"`
	SessionID       string    `json:"session_id,omitempty"` // stateful session binding
	Enabled         bool      `json:"enabled"`
	MaxRetries      int       `json:"max_retries,omitempty"`
	RetryDelayMs    int64     `json:"retry_delay_ms,omitempty"`
	TimeoutMs       int64     `json:"timeout_ms,omitempty"`
	LastRun         time.Time `json:"last_run,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	Source          string    `json:"source,omitempty"` // USER | AGENT
	SourceSessionID string    `json:"source_session_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ChatModelConfig mirrors the Python ChatModelConfig used in Session/Schedule records.
// It links a credential (by id) to a specific model + parameters.
type ChatModelConfig struct {
	Type         string         `json:"type"`          // e.g. "openai", "anthropic"
	CredentialID string         `json:"credential_id"` // references a persisted credential
	Model        string         `json:"model"`
	Parameters   map[string]any `json:"parameters,omitempty"`
}

// AgentConfig represents the persisted configuration of an agent.
type AgentConfig struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	ModelID      string         `json:"model_id"`
	ToolIDs      []string       `json:"tool_ids,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Credential stores an encrypted API key for a model provider.
type Credential struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Provider  string    `json:"provider"` // openai, anthropic, etc.
	Label     string    `json:"label"`
	Encrypted string    `json:"encrypted"` // AES-GCM encrypted API key
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StoredMessage is a persisted message within a session.
type StoredMessage struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id"`
	Role       string         `json:"role"`
	Name       string         `json:"name,omitempty"`
	Content    string         `json:"content"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Blocks     string         `json:"blocks,omitempty"` // JSON-serialized content blocks
	Usage      *message.TokenUsage `json:"usage,omitempty"`
}

// AgentSnapshot is a serialised runtime snapshot for suspend-resume.
type AgentSnapshot struct {
	SessionID  string          `json:"session_id"`
	ReplyID    string          `json:"reply_id"`
	State      *agent.AgentState `json:"state"`
	CreatedAt  time.Time       `json:"created_at"`
}
