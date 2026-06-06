package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRedisPrefix = "agentscope:state:"
const defaultRedisTTL = 24 * time.Hour

// RedisStore 是基于 Redis 的 state.Store 实现，支持 TTL 自动过期与多副本共享。
type RedisStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// RedisStoreOption 用于配置 RedisStore 的可选参数。
type RedisStoreOption func(*RedisStore)

// WithPrefix 自定义 Redis key 前缀（默认 "agentscope:state:"）。
func WithPrefix(prefix string) RedisStoreOption {
	return func(s *RedisStore) {
		s.prefix = prefix
	}
}

// WithTTL 自定义状态过期时间（默认 24 小时）。
func WithTTL(ttl time.Duration) RedisStoreOption {
	return func(s *RedisStore) {
		s.ttl = ttl
	}
}

// NewRedisStore 创建基于 Redis 的状态存储。
// 若 client 为 nil 则 panic，要求调用方保证 client 已初始化。
func NewRedisStore(client *redis.Client, opts ...RedisStoreOption) *RedisStore {
	if client == nil {
		panic("state: redis client is nil")
	}
	s := &RedisStore{
		client: client,
		prefix: defaultRedisPrefix,
		ttl:    defaultRedisTTL,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *RedisStore) redisKey(key string) string {
	return s.prefix + key
}

func (s *RedisStore) stripPrefix(redisKey string) string {
	return strings.TrimPrefix(redisKey, s.prefix)
}

// Save 将 State 序列化为 JSON 后写入 Redis，并设置 TTL。
func (s *RedisStore) Save(key string, value State) error {
	if key == "" {
		return errors.New("state: empty key")
	}
	if value == nil {
		return errors.New("state: nil value")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("state: marshal failed: %w", err)
	}
	ctx := context.Background()
	return s.client.Set(ctx, s.redisKey(key), data, s.ttl).Err()
}

// Get 从 Redis 读取并反序列化到 dest（dest 须为指向具体类型的指针，且实现 State）。
func (s *RedisStore) Get(key string, dest State) error {
	if key == "" {
		return errors.New("state: empty key")
	}
	if dest == nil {
		return errors.New("state: nil dest")
	}
	ctx := context.Background()
	data, err := s.client.Get(ctx, s.redisKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("state: key %q not found", key)
		}
		return fmt.Errorf("state: redis get failed: %w", err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("state: unmarshal failed: %w", err)
	}
	return nil
}

// Exists 判断键是否存在。
func (s *RedisStore) Exists(key string) bool {
	if key == "" {
		return false
	}
	ctx := context.Background()
	n, err := s.client.Exists(ctx, s.redisKey(key)).Result()
	return err == nil && n > 0
}

// Delete 删除键。若键不存在不返回错误。
func (s *RedisStore) Delete(key string) error {
	if key == "" {
		return errors.New("state: empty key")
	}
	ctx := context.Background()
	if err := s.client.Del(ctx, s.redisKey(key)).Err(); err != nil {
		return fmt.Errorf("state: redis del failed: %w", err)
	}
	return nil
}

// ListKeys 列出所有以 prefix 开头的键名（不含前缀）。
// 使用 SCAN 避免 KEYS 在大数据量时的阻塞风险。
func (s *RedisStore) ListKeys() []string {
	ctx := context.Background()
	var keys []string
	var cursor uint64
	match := s.redisKey("*")
	for {
		batch, nextCursor, err := s.client.Scan(ctx, cursor, match, 100).Result()
		if err != nil {
			return nil
		}
		for _, k := range batch {
			keys = append(keys, s.stripPrefix(k))
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return keys
}

// TTL 返回当前配置的默认过期时间。
func (s *RedisStore) TTL() time.Duration {
	return s.ttl
}

// Client 返回底层 redis.Client（用于高级场景，如 Watch、Pub/Sub）。
func (s *RedisStore) Client() *redis.Client {
	return s.client
}
