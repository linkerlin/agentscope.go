package service

import "context"

// Storage is the abstract persistence layer for the multi-tenant service.
// Implementations: MemoryStorage (dev/test), RedisStorage (production).
type Storage interface {
	// Users
	SaveUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	DeleteUser(ctx context.Context, id string) error

	// Sessions
	SaveSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error)
	DeleteSession(ctx context.Context, id string) error

	// Agent configs
	SaveAgentConfig(ctx context.Context, cfg *AgentConfig) error
	GetAgentConfig(ctx context.Context, id string) (*AgentConfig, error)
	ListAgentConfigsByUser(ctx context.Context, userID string) ([]*AgentConfig, error)
	DeleteAgentConfig(ctx context.Context, id string) error

	// Credentials
	SaveCredential(ctx context.Context, cred *Credential) error
	GetCredential(ctx context.Context, id string) (*Credential, error)
	ListCredentialsByUser(ctx context.Context, userID string) ([]*Credential, error)
	DeleteCredential(ctx context.Context, id string) error

	// Messages
	SaveMessage(ctx context.Context, msg *StoredMessage) error
	GetMessage(ctx context.Context, id string) (*StoredMessage, error)
	UpsertMessage(ctx context.Context, msg *StoredMessage) error
	ListMessagesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error)
	DeleteMessagesBySession(ctx context.Context, sessionID string) error

	// Agent runtime snapshots (for suspend-resume)
	SaveSnapshot(ctx context.Context, snap *AgentSnapshot) error
	GetSnapshot(ctx context.Context, sessionID string) (*AgentSnapshot, error)
	DeleteSnapshot(ctx context.Context, sessionID string) error

	// Schedules
	SaveSchedule(ctx context.Context, sched *Schedule) error
	GetSchedule(ctx context.Context, id string) (*Schedule, error)
	ListSchedulesByUser(ctx context.Context, userID string) ([]*Schedule, error)
	ListAllSchedules(ctx context.Context) ([]*Schedule, error)
	DeleteSchedule(ctx context.Context, id string) error
	ListSessionsBySchedule(ctx context.Context, userID, scheduleID string) ([]*Session, error)
}
