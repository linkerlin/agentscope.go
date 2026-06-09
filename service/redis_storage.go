package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage is a production-ready Storage implementation backed by Redis.
type RedisStorage struct {
	client redis.UniversalClient
}

// NewRedisStorage creates a RedisStorage with the given Redis client.
func NewRedisStorage(client redis.UniversalClient) *RedisStorage {
	return &RedisStorage{client: client}
}

// NewRedisStorageFromAddr creates a RedisStorage from an address string.
func NewRedisStorageFromAddr(addr, password string, db int) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return NewRedisStorage(client)
}

// Close closes the Redis client connection.
func (s *RedisStorage) Close() error {
	return s.client.Close()
}

// Ping verifies the Redis connection.
func (s *RedisStorage) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// --- Users ---

func (s *RedisStorage) SaveUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("redis: marshal user: %w", err)
	}
	return s.client.Set(ctx, keyUser(user.ID), data, 0).Err()
}

func (s *RedisStorage) GetUser(ctx context.Context, id string) (*User, error) {
	data, err := s.client.Get(ctx, keyUser(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get user: %w", err)
	}
	var u User
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		return nil, fmt.Errorf("redis: unmarshal user: %w", err)
	}
	return &u, nil
}

func (s *RedisStorage) ListUsers(ctx context.Context) ([]*User, error) {
	// Scan for all user keys (users:*)
	keys, err := s.client.Keys(ctx, "users:*").Result()
	if err != nil {
		return nil, fmt.Errorf("redis: scan users: %w", err)
	}
	if len(keys) == 0 {
		return []*User{}, nil
	}
	data, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget users: %w", err)
	}
	users := make([]*User, 0, len(data))
	for _, d := range data {
		if d == nil {
			continue
		}
		var u User
		if err := json.Unmarshal([]byte(d.(string)), &u); err != nil {
			continue
		}
		users = append(users, &u)
	}
	return users, nil
}

func (s *RedisStorage) DeleteUser(ctx context.Context, id string) error {
	return s.client.Del(ctx, keyUser(id)).Err()
}

// --- Sessions ---

func (s *RedisStorage) SaveSession(ctx context.Context, session *Session) error {
	session.UpdatedAt = time.Now()
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("redis: marshal session: %w", err)
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, keySession(session.ID), data, 0)
	pipe.SAdd(ctx, keySessionsByUser(session.UserID), session.ID)
	if session.SourceScheduleID != "" {
		pipe.SAdd(ctx, keyScheduleSessions(session.SourceScheduleID), session.ID)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: save session: %w", err)
	}
	return nil
}

func (s *RedisStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	data, err := s.client.Get(ctx, keySession(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get session: %w", err)
	}
	var se Session
	if err := json.Unmarshal([]byte(data), &se); err != nil {
		return nil, fmt.Errorf("redis: unmarshal session: %w", err)
	}
	return &se, nil
}

func (s *RedisStorage) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	ids, err := s.client.SMembers(ctx, keySessionsByUser(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list sessions: %w", err)
	}
	if len(ids) == 0 {
		return []*Session{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keySession, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget sessions: %w", err)
	}
	var out []*Session
	for _, v := range vals {
		if v == nil {
			continue
		}
		var se Session
		if err := json.Unmarshal([]byte(v.(string)), &se); err == nil {
			out = append(out, &se)
		}
	}
	return out, nil
}

func (s *RedisStorage) DeleteSession(ctx context.Context, id string) error {
	se, _ := s.GetSession(ctx, id)
	pipe := s.client.Pipeline()
	pipe.Del(ctx, keySession(id))
	pipe.Del(ctx, keyMessages(id))
	pipe.Del(ctx, keySnapshot(id))
	if se != nil {
		pipe.SRem(ctx, keySessionsByUser(se.UserID), id)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: delete session: %w", err)
	}
	return nil
}

// --- Agent Configs ---

func (s *RedisStorage) SaveAgentConfig(ctx context.Context, cfg *AgentConfig) error {
	cfg.UpdatedAt = time.Now()
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("redis: marshal agent config: %w", err)
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, keyAgent(cfg.ID), data, 0)
	pipe.SAdd(ctx, keyAgentsByUser(cfg.UserID), cfg.ID)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: save agent config: %w", err)
	}
	return nil
}

func (s *RedisStorage) GetAgentConfig(ctx context.Context, id string) (*AgentConfig, error) {
	data, err := s.client.Get(ctx, keyAgent(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("agent config not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get agent config: %w", err)
	}
	var cfg AgentConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("redis: unmarshal agent config: %w", err)
	}
	return &cfg, nil
}

func (s *RedisStorage) ListAgentConfigsByUser(ctx context.Context, userID string) ([]*AgentConfig, error) {
	ids, err := s.client.SMembers(ctx, keyAgentsByUser(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list agent configs: %w", err)
	}
	if len(ids) == 0 {
		return []*AgentConfig{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keyAgent, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget agent configs: %w", err)
	}
	var out []*AgentConfig
	for _, v := range vals {
		if v == nil {
			continue
		}
		var cfg AgentConfig
		if err := json.Unmarshal([]byte(v.(string)), &cfg); err == nil {
			out = append(out, &cfg)
		}
	}
	return out, nil
}

func (s *RedisStorage) DeleteAgentConfig(ctx context.Context, id string) error {
	cfg, _ := s.GetAgentConfig(ctx, id)
	pipe := s.client.Pipeline()
	pipe.Del(ctx, keyAgent(id))
	if cfg != nil {
		pipe.SRem(ctx, keyAgentsByUser(cfg.UserID), id)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: delete agent config: %w", err)
	}
	return nil
}

// --- Credentials ---

func (s *RedisStorage) SaveCredential(ctx context.Context, cred *Credential) error {
	cred.UpdatedAt = time.Now()
	data, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("redis: marshal credential: %w", err)
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, keyCredential(cred.ID), data, 0)
	pipe.SAdd(ctx, keyCredentialsByUser(cred.UserID), cred.ID)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: save credential: %w", err)
	}
	return nil
}

func (s *RedisStorage) GetCredential(ctx context.Context, id string) (*Credential, error) {
	data, err := s.client.Get(ctx, keyCredential(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("credential not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get credential: %w", err)
	}
	var c Credential
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		return nil, fmt.Errorf("redis: unmarshal credential: %w", err)
	}
	return &c, nil
}

func (s *RedisStorage) ListCredentialsByUser(ctx context.Context, userID string) ([]*Credential, error) {
	ids, err := s.client.SMembers(ctx, keyCredentialsByUser(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list credentials: %w", err)
	}
	if len(ids) == 0 {
		return []*Credential{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keyCredential, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget credentials: %w", err)
	}
	var out []*Credential
	for _, v := range vals {
		if v == nil {
			continue
		}
		var c Credential
		if err := json.Unmarshal([]byte(v.(string)), &c); err == nil {
			out = append(out, &c)
		}
	}
	return out, nil
}

func (s *RedisStorage) DeleteCredential(ctx context.Context, id string) error {
	cred, _ := s.GetCredential(ctx, id)
	pipe := s.client.Pipeline()
	pipe.Del(ctx, keyCredential(id))
	if cred != nil {
		pipe.SRem(ctx, keyCredentialsByUser(cred.UserID), id)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: delete credential: %w", err)
	}
	return nil
}

// --- Messages ---

func (s *RedisStorage) SaveMessage(ctx context.Context, msg *StoredMessage) error {
	msg.CreatedAt = time.Now()
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("redis: marshal message: %w", err)
	}
	return s.client.RPush(ctx, keyMessages(msg.SessionID), data).Err()
}

func (s *RedisStorage) GetMessage(ctx context.Context, id string) (*StoredMessage, error) {
	keys, err := s.client.Keys(ctx, "messages:*").Result()
	if err != nil {
		return nil, fmt.Errorf("redis: scan messages: %w", err)
	}
	for _, key := range keys {
		items, err := s.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}
		for _, item := range items {
			var m StoredMessage
			if err := json.Unmarshal([]byte(item), &m); err != nil {
				continue
			}
			if m.ID == id {
				return &m, nil
			}
		}
	}
	return nil, fmt.Errorf("message not found: %s", id)
}

func (s *RedisStorage) UpsertMessage(ctx context.Context, msg *StoredMessage) error {
	items, err := s.client.LRange(ctx, keyMessages(msg.SessionID), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("redis: lrange messages: %w", err)
	}
	for i, item := range items {
		var m StoredMessage
		if err := json.Unmarshal([]byte(item), &m); err != nil {
			continue
		}
		if m.ID == msg.ID {
			data, err := json.Marshal(msg)
			if err != nil {
				return fmt.Errorf("redis: marshal message: %w", err)
			}
			if err := s.client.LSet(ctx, keyMessages(msg.SessionID), int64(i), data).Err(); err != nil {
				return fmt.Errorf("redis: lset message: %w", err)
			}
			return nil
		}
	}
	return s.SaveMessage(ctx, msg)
}

func (s *RedisStorage) ListMessagesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error) {
	start := int64(offset)
	stop := int64(offset + limit - 1)
	items, err := s.client.LRange(ctx, keyMessages(sessionID), start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: lrange messages: %w", err)
	}
	var out []*StoredMessage
	for _, item := range items {
		var m StoredMessage
		if err := json.Unmarshal([]byte(item), &m); err == nil {
			out = append(out, &m)
		}
	}
	return out, nil
}

func (s *RedisStorage) DeleteMessagesBySession(ctx context.Context, sessionID string) error {
	return s.client.Del(ctx, keyMessages(sessionID)).Err()
}

// --- Snapshots ---

func (s *RedisStorage) SaveSnapshot(ctx context.Context, snap *AgentSnapshot) error {
	snap.CreatedAt = time.Now()
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("redis: marshal snapshot: %w", err)
	}
	return s.client.Set(ctx, keySnapshot(snap.SessionID), data, 0).Err()
}

func (s *RedisStorage) GetSnapshot(ctx context.Context, sessionID string) (*AgentSnapshot, error) {
	data, err := s.client.Get(ctx, keySnapshot(sessionID)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("snapshot not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get snapshot: %w", err)
	}
	var snap AgentSnapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return nil, fmt.Errorf("redis: unmarshal snapshot: %w", err)
	}
	return &snap, nil
}

func (s *RedisStorage) DeleteSnapshot(ctx context.Context, sessionID string) error {
	return s.client.Del(ctx, keySnapshot(sessionID)).Err()
}

// --- Schedules ---

func (s *RedisStorage) SaveSchedule(ctx context.Context, sched *Schedule) error {
	now := time.Now()
	if sched.CreatedAt.IsZero() {
		sched.CreatedAt = now
	}
	sched.UpdatedAt = now
	data, err := json.Marshal(sched)
	if err != nil {
		return fmt.Errorf("redis: marshal schedule: %w", err)
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, keySchedule(sched.ID), data, 0)
	pipe.SAdd(ctx, keySchedulesByUser(sched.UserID), sched.ID)
	pipe.SAdd(ctx, keySchedulesGlobal(), sched.ID)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: save schedule: %w", err)
	}
	return nil
}

func (s *RedisStorage) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	data, err := s.client.Get(ctx, keySchedule(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get schedule: %w", err)
	}
	var sched Schedule
	if err := json.Unmarshal([]byte(data), &sched); err != nil {
		return nil, fmt.Errorf("redis: unmarshal schedule: %w", err)
	}
	return &sched, nil
}

func (s *RedisStorage) ListSchedulesByUser(ctx context.Context, userID string) ([]*Schedule, error) {
	ids, err := s.client.SMembers(ctx, keySchedulesByUser(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list schedules: %w", err)
	}
	if len(ids) == 0 {
		return []*Schedule{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keySchedule, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget schedules: %w", err)
	}
	var out []*Schedule
	for _, v := range vals {
		if v == nil {
			continue
		}
		var sched Schedule
		if err := json.Unmarshal([]byte(v.(string)), &sched); err != nil {
			continue
		}
		out = append(out, &sched)
	}
	return out, nil
}

func (s *RedisStorage) ListAllSchedules(ctx context.Context) ([]*Schedule, error) {
	ids, err := s.client.SMembers(ctx, keySchedulesGlobal()).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list all schedules: %w", err)
	}
	if len(ids) == 0 {
		return []*Schedule{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keySchedule, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget all schedules: %w", err)
	}
	var out []*Schedule
	for _, v := range vals {
		if v == nil {
			continue
		}
		var sched Schedule
		if err := json.Unmarshal([]byte(v.(string)), &sched); err != nil {
			continue
		}
		out = append(out, &sched)
	}
	return out, nil
}

func (s *RedisStorage) DeleteSchedule(ctx context.Context, id string) error {
	sched, _ := s.GetSchedule(ctx, id)
	pipe := s.client.Pipeline()
	pipe.Del(ctx, keySchedule(id))
	if sched != nil {
		pipe.SRem(ctx, keySchedulesByUser(sched.UserID), id)
	}
	pipe.SRem(ctx, keySchedulesGlobal(), id)
	pipe.Del(ctx, keyScheduleSessions(id))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: delete schedule: %w", err)
	}
	return nil
}

func (s *RedisStorage) ListSessionsBySchedule(ctx context.Context, userID, scheduleID string) ([]*Session, error) {
	sched, err := s.GetSchedule(ctx, scheduleID)
	if err != nil || sched.UserID != userID {
		return nil, fmt.Errorf("schedule not found: %s", scheduleID)
	}
	ids, err := s.client.SMembers(ctx, keyScheduleSessions(scheduleID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list schedule sessions: %w", err)
	}
	if len(ids) == 0 {
		return []*Session{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keySession, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget schedule sessions: %w", err)
	}
	var out []*Session
	for _, v := range vals {
		if v == nil {
			continue
		}
		var se Session
		if err := json.Unmarshal([]byte(v.(string)), &se); err != nil {
			continue
		}
		if se.UserID == userID {
			out = append(out, &se)
		}
	}
	return out, nil
}

// --- key helpers ---

func keyUser(id string) string        { return fmt.Sprintf("users:%s", id) }
func keySession(id string) string     { return fmt.Sprintf("sessions:%s", id) }
func keySessionsByUser(uid string) string {
	return fmt.Sprintf("sessions_by_user:%s", uid)
}
func keyAgent(id string) string       { return fmt.Sprintf("agents:%s", id) }
func keyAgentsByUser(uid string) string {
	return fmt.Sprintf("agents_by_user:%s", uid)
}
func keyCredential(id string) string  { return fmt.Sprintf("credentials:%s", id) }
func keyCredentialsByUser(uid string) string {
	return fmt.Sprintf("credentials_by_user:%s", uid)
}
func keyMessages(sessionID string) string {
	return fmt.Sprintf("messages:%s", sessionID)
}
func keySnapshot(sessionID string) string {
	return fmt.Sprintf("snapshots:%s", sessionID)
}
func keySchedule(id string) string { return fmt.Sprintf("schedules:%s", id) }
func keySchedulesByUser(uid string) string {
	return fmt.Sprintf("schedules_by_user:%s", uid)
}
func keySchedulesGlobal() string { return "schedules:all" }
func keyScheduleSessions(scheduleID string) string {
	return fmt.Sprintf("schedule_sessions:%s", scheduleID)
}

func makeKeys(fn func(string) string, ids []string) []string {
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fn(id)
	}
	return keys
}

// Compile-time check.
var _ Storage = (*RedisStorage)(nil)
