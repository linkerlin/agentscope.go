package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRegistryStore persists A2A registry entries in Redis.
// Each entry is stored as a JSON string under a per-URL key, and an index set
// keeps track of all registered URLs.
type RedisRegistryStore struct {
	client    redis.UniversalClient
	keyPrefix string
	ttl       time.Duration
}

// NewRedisRegistryStore creates a Redis-backed registry store.
// keyPrefix defaults to "a2a:registry". ttl defaults to 24 hours.
func NewRedisRegistryStore(client redis.UniversalClient, keyPrefix string) *RedisRegistryStore {
	if keyPrefix == "" {
		keyPrefix = "a2a:registry"
	}
	return &RedisRegistryStore{
		client:    client,
		keyPrefix: keyPrefix,
		ttl:       24 * time.Hour,
	}
}

func (s *RedisRegistryStore) entryKey(url string) string {
	return fmt.Sprintf("%s:entry:%s", s.keyPrefix, url)
}

func (s *RedisRegistryStore) indexKey() string {
	return s.keyPrefix + ":urls"
}

// Register persists an entry and adds its URL to the index.
func (s *RedisRegistryStore) Register(ctx context.Context, entry RegistryEntry) error {
	if entry.Card.URL == "" {
		return fmt.Errorf("a2a registry: card URL is required")
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.entryKey(entry.Card.URL), data, s.ttl)
	pipe.SAdd(ctx, s.indexKey(), entry.Card.URL)
	pipe.Expire(ctx, s.indexKey(), s.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

// Get returns a single entry by URL.
func (s *RedisRegistryStore) Get(ctx context.Context, url string) (*RegistryEntry, error) {
	data, err := s.client.Get(ctx, s.entryKey(url)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entry RegistryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// List returns all entries currently in the index.
func (s *RedisRegistryStore) List(ctx context.Context) ([]RegistryEntry, error) {
	urls, err := s.client.SMembers(ctx, s.indexKey()).Result()
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, nil
	}
	keys := make([]string, len(urls))
	for i, url := range urls {
		keys[i] = s.entryKey(url)
	}
	values, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]RegistryEntry, 0, len(values))
	for i, v := range values {
		if v == nil {
			continue
		}
		str, ok := v.(string)
		if !ok {
			continue
		}
		var entry RegistryEntry
		if err := json.Unmarshal([]byte(str), &entry); err != nil {
			continue
		}
		// Defensive: ensure the stored URL matches the index in case of key reuse.
		if entry.Card.URL == "" {
			entry.Card.URL = urls[i]
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Remove deletes an entry and its URL from the index.
func (s *RedisRegistryStore) Remove(ctx context.Context, url string) error {
	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.entryKey(url))
	pipe.SRem(ctx, s.indexKey(), url)
	_, err := pipe.Exec(ctx)
	return err
}

// UpdateHealth updates the healthy flag and last-seen timestamp of an entry.
func (s *RedisRegistryStore) UpdateHealth(ctx context.Context, url string, healthy bool, lastSeen time.Time) error {
	entry, err := s.Get(ctx, url)
	if err != nil {
		return err
	}
	if entry == nil {
		return nil
	}
	entry.Healthy = healthy
	entry.LastSeen = lastSeen
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.entryKey(url), data, s.ttl).Err()
}

var _ RegistryStore = (*RedisRegistryStore)(nil)
