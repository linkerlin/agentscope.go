package state

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return mr, rdb
}

func TestRedisStoreRoundTrip(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	if err := s.Save("k1", testState{V: "hello"}); err != nil {
		t.Fatal(err)
	}
	if !s.Exists("k1") {
		t.Fatal("expected exists")
	}

	var got testState
	if err := s.Get("k1", &got); err != nil {
		t.Fatal(err)
	}
	if got.V != "hello" {
		t.Fatalf("got %q", got.V)
	}

	keys := s.ListKeys()
	if len(keys) != 1 || keys[0] != "k1" {
		t.Fatalf("expected [k1], got %v", keys)
	}

	if err := s.Delete("k1"); err != nil {
		t.Fatal(err)
	}
	if s.Exists("k1") {
		t.Fatal("expected deleted")
	}
}

func TestRedisStoreGetNotFound(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	var got testState
	if err := s.Get("missing", &got); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestRedisStoreDeleteNotExists(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	// 删除不存在的 key 不应报错
	if err := s.Delete("no_such_key"); err != nil {
		t.Fatal(err)
	}
}

func TestRedisStoreSaveErrors(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	if err := s.Save("", testState{V: "x"}); err == nil {
		t.Fatal("expected error for empty key")
	}
	if err := s.Save("k", nil); err == nil {
		t.Fatal("expected error for nil value")
	}
}

func TestRedisStoreGetErrors(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	if err := s.Get("", &testState{}); err == nil {
		t.Fatal("expected error for empty key")
	}
	if err := s.Get("k", nil); err == nil {
		t.Fatal("expected error for nil dest")
	}
}

func TestRedisStoreExistsEmptyKey(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)
	if s.Exists("") {
		t.Fatal("expected false for empty key")
	}
}

func TestRedisStoreDeleteEmptyKey(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)
	if err := s.Delete(""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestRedisStoreListKeysEmpty(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)
	keys := s.ListKeys()
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

func TestRedisStoreCustomPrefixAndTTL(t *testing.T) {
	mr, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb,
		WithPrefix("custom:prefix:"),
		WithTTL(5*time.Minute),
	)

	if s.TTL() != 5*time.Minute {
		t.Fatalf("expected ttl 5m, got %v", s.TTL())
	}

	_ = s.Save("k1", testState{V: "prefixed"})

	// 验证 key 确实使用了自定义前缀
	if !mr.Exists("custom:prefix:k1") {
		t.Fatal("expected key with custom prefix to exist in redis")
	}

	// 验证 TTL
	mr.FastForward(6 * time.Minute)
	if s.Exists("k1") {
		t.Fatal("expected key expired after ttl")
	}
}

func TestRedisStoreDefaultTTL(t *testing.T) {
	mr, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	_ = s.Save("k1", testState{V: "ttl_test"})

	// 默认 TTL 24h，快进 23h 应仍存在
	mr.FastForward(23 * time.Hour)
	if !s.Exists("k1") {
		t.Fatal("expected key still exists before ttl")
	}

	// 再快进 2h 应过期
	mr.FastForward(2 * time.Hour)
	if s.Exists("k1") {
		t.Fatal("expected key expired after default ttl")
	}
}

func TestRedisStoreListKeysMultiple(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)

	_ = s.Save("a", testState{V: "1"})
	_ = s.Save("b", testState{V: "2"})
	_ = s.Save("c", testState{V: "3"})

	keys := s.ListKeys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %v", keys)
	}

	// 写入一个非 state 前缀的 key，不应被 ListKeys 返回
	_ = s.Client().Set(context.Background(), "other:key", "x", 0).Err()
	keys = s.ListKeys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys after adding unrelated key, got %v", keys)
	}
}

func TestRedisStoreClientAccessor(t *testing.T) {
	_, rdb := setupMiniredis(t)
	s := NewRedisStore(rdb)
	if s.Client() != rdb {
		t.Fatal("expected Client() to return underlying redis client")
	}
}

func TestRedisStoreNilClientPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil client")
		}
	}()
	_ = NewRedisStore(nil)
}
