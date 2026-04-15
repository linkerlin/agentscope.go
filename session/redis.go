package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/linkerlin/agentscope.go/message"
)

// RedisSessionService stores sessions in Redis with JSON serialization.
type RedisSessionService struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisSessionService creates a Redis-backed session service.
// Prefix is prepended to every Redis key. TTL <= 0 means no expiration.
func NewRedisSessionService(client *redis.Client, prefix string, ttl time.Duration) *RedisSessionService {
	if prefix == "" {
		prefix = "agentscope:session:"
	}
	return &RedisSessionService{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}
}

func (s *RedisSessionService) key(sessionID string) string {
	return s.prefix + sessionID
}

// redisSession is a serializable snapshot of a Session.
type redisSession struct {
	ID        string          `json:"id"`
	AgentName string          `json:"agent_name"`
	Messages  []*message.Msg  `json:"messages"`
	Metadata  map[string]any  `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (s *RedisSessionService) Create(agentName string) (*Session, error) {
	sess := &Session{
		ID:        uuid.New().String(),
		AgentName: agentName,
		Metadata:  make(map[string]any),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.save(context.Background(), sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *RedisSessionService) Get(sessionID string) (*Session, error) {
	ctx := context.Background()
	data, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	return s.decode(data)
}

func (s *RedisSessionService) AddMessage(sessionID string, msg *message.Msg) error {
	ctx := context.Background()
	sess, err := s.Get(sessionID)
	if err != nil {
		return err
	}
	sess.mu.Lock()
	sess.Messages = append(sess.Messages, msg)
	sess.UpdatedAt = time.Now()
	sess.mu.Unlock()
	return s.save(ctx, sess)
}

func (s *RedisSessionService) Delete(sessionID string) error {
	ctx := context.Background()
	err := s.client.Del(ctx, s.key(sessionID)).Err()
	if err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

func (s *RedisSessionService) List() ([]*Session, error) {
	ctx := context.Background()
	var cursor uint64
	var keys []string
	for {
		var batch []string
		var err error
		batch, cursor, err = s.client.Scan(ctx, cursor, s.prefix+"*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	sessions := make([]*Session, 0, len(keys))
	for _, k := range keys {
		data, err := s.client.Get(ctx, k).Bytes()
		if err != nil {
			continue
		}
		sess, err := s.decode(data)
		if err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (s *RedisSessionService) save(ctx context.Context, sess *Session) error {
	snap := redisSession{
		ID:        sess.ID,
		AgentName: sess.AgentName,
		Messages:  append([]*message.Msg(nil), sess.Messages...),
		Metadata:  cloneMap(sess.Metadata),
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	key := s.key(sess.ID)
	if s.ttl > 0 {
		err = s.client.Set(ctx, key, data, s.ttl).Err()
	} else {
		err = s.client.Set(ctx, key, data, 0).Err()
	}
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (s *RedisSessionService) decode(data []byte) (*Session, error) {
	var snap redisSession
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	sess := &Session{
		ID:        snap.ID,
		AgentName: snap.AgentName,
		Messages:  snap.Messages,
		Metadata:  cloneMap(snap.Metadata),
		CreatedAt: snap.CreatedAt,
		UpdatedAt: snap.UpdatedAt,
	}
	return sess, nil
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
