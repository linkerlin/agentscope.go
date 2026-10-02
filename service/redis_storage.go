package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func (s *RedisStorage) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	users, err := s.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found by email: %s", email)
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
	data, err := json.Marshal(credentialToPersist(cred))
	if err != nil {
		return fmt.Errorf("redis: marshal credential: %w", err)
	}
	// Rotation: drop the old hash index entry when the credential moves to a
	// different key (or away from api_key entirely).
	old, _ := s.GetCredential(ctx, cred.ID)
	newHash := apiKeyIndexKey(cred)
	pipe := s.client.Pipeline()
	pipe.Set(ctx, keyCredential(cred.ID), data, 0)
	pipe.SAdd(ctx, keyCredentialsByUser(cred.UserID), cred.ID)
	if newHash != "" {
		pipe.SAdd(ctx, keyCredentialsByHash(newHash), cred.ID)
	}
	if old != nil {
		if oldHash := apiKeyIndexKey(old); oldHash != "" && oldHash != newHash {
			pipe.SRem(ctx, keyCredentialsByHash(oldHash), cred.ID)
		}
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: save credential: %w", err)
	}
	return nil
}

// redisCredentialCAS is one atomic conditional write: KEYS[1] is the
// credential key; ARGV[1] the new payload JSON, ARGV[2] the allowed-status
// list (comma-joined), ARGV[3] the hash-index key prefix, ARGV[4] the
// credential id, ARGV[5] the by-user set key. Replies {0, <current payload>}
// when the condition fails and {1, ""} on success.
var redisCredentialCAS = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if not cur then return {0, ''} end
local st = cjson.decode(cur)['status']
-- Empty/missing status (pre-18.4 rows) normalizes to AUTHORIZED — the same
-- rule as NormalizedStatus and the Memory backend: never an unconditional
-- pass, only allowed when the caller explicitly allows AUTHORIZED.
if st == nil or st == '' then st = 'AUTHORIZED' end
local okStatus = false
for allowed in string.gmatch(ARGV[2], '[^,]+') do
  if st == allowed then okStatus = true end
end
if not okStatus then return {0, cur} end
local oldEnc = cjson.decode(cur)['encrypted']
if oldEnc and string.sub(oldEnc, 1, 7) == 'sha256:' then
  redis.call('SREM', ARGV[3] .. oldEnc, ARGV[4])
end
redis.call('SET', KEYS[1], ARGV[1])
local newEnc = cjson.decode(ARGV[1])['encrypted']
if newEnc and string.sub(newEnc, 1, 7) == 'sha256:' then
  redis.call('SADD', ARGV[3] .. newEnc, ARGV[4])
end
redis.call('SADD', ARGV[5], ARGV[4])
return {1, ''}
`)

// SaveCredentialIfCurrent implements CredentialConditionalWriter (18.4
// review fix): GET + judge + SET + index maintenance in one Lua script, so
// racing writers across processes cannot both pass the check.
func (s *RedisStorage) SaveCredentialIfCurrent(ctx context.Context, id string, next *Credential, allowedCurrent ...CredentialStatus) (*Credential, bool, error) {
	if len(allowedCurrent) == 0 {
		cur, err := s.GetCredential(ctx, id)
		return cur, false, err
	}
	next.UpdatedAt = time.Now()
	data, err := json.Marshal(credentialToPersist(next))
	if err != nil {
		return nil, false, fmt.Errorf("redis: marshal credential: %w", err)
	}
	parts := make([]string, len(allowedCurrent))
	for i, a := range allowedCurrent {
		parts[i] = string(a)
	}
	res, err := redisCredentialCAS.Run(ctx, s.client,
		[]string{keyCredential(id)},
		string(data), strings.Join(parts, ","), "credentials_by_hash:", id, keyCredentialsByUser(next.UserID),
	).Slice()
	if err != nil {
		return nil, false, fmt.Errorf("redis: conditional credential save: %w", err)
	}
	swapped, _ := res[0].(int64)
	if swapped == 1 {
		return next, true, nil
	}
	curPayload, _ := res[1].(string)
	if curPayload == "" {
		return nil, false, fmt.Errorf("credential not found: %s", id)
	}
	var row credentialPersist
	if err := json.Unmarshal([]byte(curPayload), &row); err != nil {
		return nil, false, err
	}
	return row.toCredential(), false, nil
}

func (s *RedisStorage) GetCredential(ctx context.Context, id string) (*Credential, error) {
	data, err := s.client.Get(ctx, keyCredential(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("credential not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get credential: %w", err)
	}
	var row credentialPersist
	if err := json.Unmarshal([]byte(data), &row); err != nil {
		return nil, fmt.Errorf("redis: unmarshal credential: %w", err)
	}
	return row.toCredential(), nil
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
		var row credentialPersist
		if err := json.Unmarshal([]byte(v.(string)), &row); err == nil {
			out = append(out, row.toCredential())
		}
	}
	return out, nil
}

// FindCredentialsByHash implements APIKeyCredentialFinder: one SET lookup
// (credentials_by_hash:<hash>) replaces the per-user scan in
// FindUserByAPIKey.
func (s *RedisStorage) FindCredentialsByHash(ctx context.Context, keyHash string) ([]*Credential, error) {
	ids, err := s.client.SMembers(ctx, keyCredentialsByHash(keyHash)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: find credentials by hash: %w", err)
	}
	if len(ids) == 0 {
		return []*Credential{}, nil
	}
	vals, err := s.client.MGet(ctx, makeKeys(keyCredential, ids)...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: mget credentials by hash: %w", err)
	}
	out := make([]*Credential, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		var row credentialPersist
		if err := json.Unmarshal([]byte(v.(string)), &row); err == nil {
			out = append(out, row.toCredential())
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
		if k := apiKeyIndexKey(cred); k != "" {
			pipe.SRem(ctx, keyCredentialsByHash(k), id)
		}
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

func keyUser(id string) string    { return fmt.Sprintf("users:%s", id) }
func keySession(id string) string { return fmt.Sprintf("sessions:%s", id) }
func keySessionsByUser(uid string) string {
	return fmt.Sprintf("sessions_by_user:%s", uid)
}
func keyAgent(id string) string { return fmt.Sprintf("agents:%s", id) }
func keyAgentsByUser(uid string) string {
	return fmt.Sprintf("agents_by_user:%s", uid)
}
func keyCredential(id string) string { return fmt.Sprintf("credentials:%s", id) }
func keyCredentialsByUser(uid string) string {
	return fmt.Sprintf("credentials_by_user:%s", uid)
}
func keyCredentialsByHash(hash string) string {
	return fmt.Sprintf("credentials_by_hash:%s", hash)
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
func keyTeam(id string) string          { return fmt.Sprintf("teams:%s", id) }
func keyTeamsByUser(uid string) string  { return fmt.Sprintf("teams_by_user:%s", uid) }
func keyTeamByLeader(sid string) string { return fmt.Sprintf("team_by_leader:%s", sid) }

func makeKeys(fn func(string) string, ids []string) []string {
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fn(id)
	}
	return keys
}

// --- Teams ---

func (s *RedisStorage) SaveTeam(ctx context.Context, team *Team) error {
	team.UpdatedAt = time.Now()
	if team.CreatedAt.IsZero() {
		team.CreatedAt = team.UpdatedAt
	}
	data, err := json.Marshal(team)
	if err != nil {
		return fmt.Errorf("redis: marshal team: %w", err)
	}
	if err := s.client.Set(ctx, keyTeam(team.ID), data, 0).Err(); err != nil {
		return err
	}
	if team.UserID != "" {
		s.client.SAdd(ctx, keyTeamsByUser(team.UserID), team.ID)
	}
	if team.LeaderSessionID != "" {
		s.client.Set(ctx, keyTeamByLeader(team.LeaderSessionID), team.ID, 0)
	}
	return nil
}

func (s *RedisStorage) GetTeam(ctx context.Context, id string) (*Team, error) {
	data, err := s.client.Get(ctx, keyTeam(id)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("team not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get team: %w", err)
	}
	var t Team
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, fmt.Errorf("redis: unmarshal team: %w", err)
	}
	return &t, nil
}

func (s *RedisStorage) ListTeamsByUser(ctx context.Context, userID string) ([]*Team, error) {
	ids, err := s.client.SMembers(ctx, keyTeamsByUser(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: list teams: %w", err)
	}
	out := make([]*Team, 0, len(ids))
	for _, id := range ids {
		if t, err := s.GetTeam(ctx, id); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *RedisStorage) DeleteTeam(ctx context.Context, id string) error {
	t, err := s.GetTeam(ctx, id)
	if err != nil {
		return nil
	}
	s.client.Del(ctx, keyTeam(id))
	if t.UserID != "" {
		s.client.SRem(ctx, keyTeamsByUser(t.UserID), id)
	}
	if t.LeaderSessionID != "" {
		s.client.Del(ctx, keyTeamByLeader(t.LeaderSessionID))
	}
	return nil
}

func (s *RedisStorage) GetTeamByLeaderSession(ctx context.Context, sessionID string) (*Team, error) {
	id, err := s.client.Get(ctx, keyTeamByLeader(sessionID)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("no team led by session: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get team by leader: %w", err)
	}
	return s.GetTeam(ctx, id)
}

// Compile-time checks.
var (
	_ Storage                = (*RedisStorage)(nil)
	_ APIKeyCredentialFinder = (*RedisStorage)(nil)
)
