package service

import (
	"context"
	"fmt"
)

// RedisStorage is a production-ready Storage implementation backed by Redis.
// This is a skeleton; the full Redis client integration can be added later
// without changing the Storage interface.
type RedisStorage struct {
	// TODO: add redis.UniversalClient
	addr     string
	password string
	db       int
}

// NewRedisStorage creates a RedisStorage skeleton.
func NewRedisStorage(addr, password string, db int) *RedisStorage {
	return &RedisStorage{addr: addr, password: password, db: db}
}

func (s *RedisStorage) SaveUser(ctx context.Context, user *User) error {
	return fmt.Errorf("redis storage: SaveUser not yet implemented")
}

func (s *RedisStorage) GetUser(ctx context.Context, id string) (*User, error) {
	return nil, fmt.Errorf("redis storage: GetUser not yet implemented")
}

func (s *RedisStorage) DeleteUser(ctx context.Context, id string) error {
	return fmt.Errorf("redis storage: DeleteUser not yet implemented")
}

func (s *RedisStorage) SaveSession(ctx context.Context, session *Session) error {
	return fmt.Errorf("redis storage: SaveSession not yet implemented")
}

func (s *RedisStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	return nil, fmt.Errorf("redis storage: GetSession not yet implemented")
}

func (s *RedisStorage) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	return nil, fmt.Errorf("redis storage: ListSessionsByUser not yet implemented")
}

func (s *RedisStorage) DeleteSession(ctx context.Context, id string) error {
	return fmt.Errorf("redis storage: DeleteSession not yet implemented")
}

func (s *RedisStorage) SaveAgentConfig(ctx context.Context, cfg *AgentConfig) error {
	return fmt.Errorf("redis storage: SaveAgentConfig not yet implemented")
}

func (s *RedisStorage) GetAgentConfig(ctx context.Context, id string) (*AgentConfig, error) {
	return nil, fmt.Errorf("redis storage: GetAgentConfig not yet implemented")
}

func (s *RedisStorage) ListAgentConfigsByUser(ctx context.Context, userID string) ([]*AgentConfig, error) {
	return nil, fmt.Errorf("redis storage: ListAgentConfigsByUser not yet implemented")
}

func (s *RedisStorage) DeleteAgentConfig(ctx context.Context, id string) error {
	return fmt.Errorf("redis storage: DeleteAgentConfig not yet implemented")
}

func (s *RedisStorage) SaveCredential(ctx context.Context, cred *Credential) error {
	return fmt.Errorf("redis storage: SaveCredential not yet implemented")
}

func (s *RedisStorage) GetCredential(ctx context.Context, id string) (*Credential, error) {
	return nil, fmt.Errorf("redis storage: GetCredential not yet implemented")
}

func (s *RedisStorage) ListCredentialsByUser(ctx context.Context, userID string) ([]*Credential, error) {
	return nil, fmt.Errorf("redis storage: ListCredentialsByUser not yet implemented")
}

func (s *RedisStorage) DeleteCredential(ctx context.Context, id string) error {
	return fmt.Errorf("redis storage: DeleteCredential not yet implemented")
}

func (s *RedisStorage) SaveMessage(ctx context.Context, msg *StoredMessage) error {
	return fmt.Errorf("redis storage: SaveMessage not yet implemented")
}

func (s *RedisStorage) ListMessagesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error) {
	return nil, fmt.Errorf("redis storage: ListMessagesBySession not yet implemented")
}

func (s *RedisStorage) DeleteMessagesBySession(ctx context.Context, sessionID string) error {
	return fmt.Errorf("redis storage: DeleteMessagesBySession not yet implemented")
}

func (s *RedisStorage) SaveSnapshot(ctx context.Context, snap *AgentSnapshot) error {
	return fmt.Errorf("redis storage: SaveSnapshot not yet implemented")
}

func (s *RedisStorage) GetSnapshot(ctx context.Context, sessionID string) (*AgentSnapshot, error) {
	return nil, fmt.Errorf("redis storage: GetSnapshot not yet implemented")
}

func (s *RedisStorage) DeleteSnapshot(ctx context.Context, sessionID string) error {
	return fmt.Errorf("redis storage: DeleteSnapshot not yet implemented")
}

// Compile-time check.
var _ Storage = (*RedisStorage)(nil)
