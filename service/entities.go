package service

import (
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
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
	TeamID           string    `json:"team_id,omitempty"` // team membership (leader or worker session)
	Source           string    `json:"source,omitempty"`  // "user" (default) or "team" (worker)
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

// SubagentTemplate describes a child agent spawned (as a tool) by a leader
// agent. Aligns with Python agentscope's agent-team custom subagent templates
// (#1833). The leader inherits its permission context to spawned subagents
// (#1815) at build time.
type SubagentTemplate struct {
	Name         string         `json:"name"`
	ModelID      string         `json:"model_id,omitempty"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	Description  string         `json:"description,omitempty"`
	ToolIDs      []string       `json:"tool_ids,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// AgentConfig represents the persisted configuration of an agent.
type AgentConfig struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	ModelID      string `json:"model_id"`
	// AgentClass selects the agent construction strategy (default "react").
	// Custom classes are registered on the gateway's AgentFactory via
	// RegisterAgentClass. Aligns with Python agentscope's custom agent class
	// support (#1838).
	AgentClass string `json:"agent_class,omitempty"`
	// SubagentTemplates describe child agents a leader agent spawns as tools.
	SubagentTemplates []SubagentTemplate `json:"subagent_templates,omitempty"`
	ToolIDs           []string           `json:"tool_ids,omitempty"`
	Metadata          map[string]any     `json:"metadata,omitempty"`
	// Source is "user" (default, visible in global agent lists) or "team"
	// (a worker spawned by a leader — hidden from global lists). Aligns with
	// Python agentscope's agent source field for team workers.
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Credential stores an encrypted API key for a model provider.
type Credential struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Provider string `json:"provider"` // openai, anthropic, etc.
	Label    string `json:"label"`
	// Encrypted never leaves the server: it holds an AES-GCM ciphertext (or a
	// "sha256:" API-key digest) and is omitted from every JSON response so
	// GET/LIST credentials cannot leak secrets (22.1).
	Encrypted string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Status is the interactive-binding state machine (18.4): credentials can
	// be created PENDING (binding initiated, secret not yet delivered) and
	// move to AUTHORIZED / FAILED / CANCELLED via idempotent transitions.
	// Empty reads as AUTHORIZED — pre-18.4 rows were created usable and stay
	// usable unchanged.
	Status CredentialStatus `json:"status,omitempty"`
	// BindingRef carries the interactive binding's external reference (an
	// authorization-flow ID, a platform union id, …). A reference only:
	// interaction payloads and secrets never land on this field (18.4).
	BindingRef string `json:"binding_ref,omitempty"`
}

// CredentialStatus is the interactive-binding lifecycle (18.4). The zero
// value "" normalizes to CredentialAuthorized for backward compatibility.
type CredentialStatus string

const (
	// CredentialPending: the binding was initiated but the secret has not
	// been delivered. Interactive transitions may still move it anywhere.
	CredentialPending CredentialStatus = "PENDING"
	// CredentialAuthorized: the credential is complete and usable. Terminal.
	CredentialAuthorized CredentialStatus = "AUTHORIZED"
	// CredentialFailed: the binding flow reported failure. Terminal.
	CredentialFailed CredentialStatus = "FAILED"
	// CredentialCancelled: the user cancelled the binding. Terminal.
	CredentialCancelled CredentialStatus = "CANCELLED"
)

// NormalizedStatus returns the effective status: "" reads as AUTHORIZED so
// credentials written before 18.4 keep their semantics.
func (c *Credential) NormalizedStatus() CredentialStatus {
	if c == nil || c.Status == "" {
		return CredentialAuthorized
	}
	return c.Status
}

// credentialPersist mirrors Credential with the secret included in its
// serialized form. Storage backends (SQL payload column, Redis values) use it
// for persistence only — the wire type omits Encrypted, so routing storage
// through json.Marshal(Credential) would silently drop the secret. The
// serialized shape matches the historical payload, so existing rows load
// unchanged.
type credentialPersist struct {
	ID         string           `json:"id"`
	UserID     string           `json:"user_id"`
	Provider   string           `json:"provider"`
	Label      string           `json:"label"`
	Encrypted  string           `json:"encrypted"`
	Status     CredentialStatus `json:"status,omitempty"`
	BindingRef string           `json:"binding_ref,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

func credentialToPersist(c *Credential) credentialPersist {
	return credentialPersist{
		ID: c.ID, UserID: c.UserID, Provider: c.Provider, Label: c.Label,
		Encrypted: c.Encrypted, Status: c.Status, BindingRef: c.BindingRef,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func (p credentialPersist) toCredential() *Credential {
	return &Credential{
		ID: p.ID, UserID: p.UserID, Provider: p.Provider, Label: p.Label,
		Encrypted: p.Encrypted, Status: p.Status, BindingRef: p.BindingRef,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// StoredMessage is a persisted message within a session.
type StoredMessage struct {
	ID         string              `json:"id"`
	SessionID  string              `json:"session_id"`
	Role       string              `json:"role"`
	Name       string              `json:"name,omitempty"`
	Content    string              `json:"content"`
	Metadata   map[string]any      `json:"metadata,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
	Blocks     string              `json:"blocks,omitempty"` // JSON-serialized content blocks
	Usage      *message.TokenUsage `json:"usage,omitempty"`
}

// AgentSnapshot is a serialised runtime snapshot for suspend-resume.
type AgentSnapshot struct {
	SessionID string            `json:"session_id"`
	ReplyID   string            `json:"reply_id"`
	State     *agent.AgentState `json:"state"`
	CreatedAt time.Time         `json:"created_at"`
	// PendingResume carries the persisted HITL resume command (23.2). It
	// lives on the snapshot so command persistence, versioning and the
	// post-completion delete share one atomic storage record.
	PendingResume *ResumeCommand `json:"pending_resume,omitempty"`
}

// ResumeCommand states.
const (
	// ResumePending: the command is persisted but not yet delivered to a
	// live agent waiter.
	ResumePending = "pending"
	// ResumeExecuting: the command was delivered (waiter signalled); the
	// tool it resumes is running. Further confirms for the same ID are
	// refused — the tool executes at most once.
	ResumeExecuting = "executing"
)

// ResumeCommand is a versioned, idempotent HITL resume command (23.2).
// ConfirmID is the idempotency key; Version is monotonic per session so
// concurrent replicas can order commands.
type ResumeCommand struct {
	ConfirmID  string                  `json:"confirm_id"`
	ReplyID    string                  `json:"reply_id"`
	Decisions  []event.ConfirmDecision `json:"decisions,omitempty"`
	Version    int64                   `json:"version"`
	State      string                  `json:"state"`
	CreatedAt  time.Time               `json:"created_at"`
	ExecutedAt *time.Time              `json:"executed_at,omitempty"`
}

// TeamMember records a worker agent within a team, carrying the routing info
// (name + session id) needed by TeamSay without extra storage lookups.
type TeamMember struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	SessionID string `json:"session_id"`
}

// Team mirrors Python agentscope's TeamRecord + TeamData: a leader session
// coordinates a set of independent worker agents (each with its own session)
// via asynchronous messaging through the message bus. The leader is identified
// by LeaderSessionID; workers are listed in Members. Each worker agent has
// Source="team" and its session carries the same TeamID.
type Team struct {
	ID              string       `json:"id"`
	UserID          string       `json:"user_id"`
	LeaderSessionID string       `json:"leader_session_id"`
	Name            string       `json:"name"`
	Description     string       `json:"description,omitempty"`
	Members         []TeamMember `json:"members,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}
