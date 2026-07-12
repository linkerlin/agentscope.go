package messagebus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// lockReleaseScript deletes a lock key only if it still holds the caller's
// token, preventing a stale holder from releasing a lock someone else acquired.
const lockReleaseScript = `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`

// lockPollInterval is how often Lock retries the SET NX when the key is held.
const lockPollInterval = 50 * time.Millisecond

func (b *RedisBus) lockKey(resource string) string { return b.prefix + ":lock:" + resource }
func (b *RedisBus) queueKey(name string) string    { return b.prefix + ":queue:" + name }
func (b *RedisBus) hashKey(ns string) string       { return b.prefix + ":hash:" + ns }
func (b *RedisBus) logKey(ns string) string        { return b.prefix + ":log:" + ns }

func randomToken() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (b *RedisBus) Lock(ctx context.Context, key string, ttl time.Duration) (Cancel, error) {
	if b.client == nil {
		return nil, ErrClosed
	}
	rkey := b.lockKey(key)
	token := randomToken()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		args := &redis.SetArgs{Mode: "NX"}
		if ttl > 0 {
			args.TTL = ttl
		}
		ok, err := b.client.SetArgs(ctx, rkey, token, *args).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		if ok == "OK" {
			var once bool
			return Cancel(func() {
				if once {
					return
				}
				once = true
				// ponytail: ignore release error — best-effort cleanup.
				_ = b.client.Eval(context.Background(), lockReleaseScript, []string{rkey}, token).Err()
				if ttl > 0 {
					// also schedule nothing; token-gated DEL is authoritative
				}
			}), nil
		}
		select {
		case <-time.After(lockPollInterval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (b *RedisBus) RegistrySet(ctx context.Context, ns, key string, value []byte) error {
	if b.client == nil {
		return ErrClosed
	}
	hkey := b.hashKey(ns)
	if len(value) == 0 {
		return b.client.HDel(ctx, hkey, key).Err()
	}
	return b.client.HSet(ctx, hkey, key, value).Err()
}

func (b *RedisBus) RegistryGet(ctx context.Context, ns, key string) ([]byte, error) {
	if b.client == nil {
		return nil, ErrClosed
	}
	v, err := b.client.HGet(ctx, b.hashKey(ns), key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return v, nil
}

func (b *RedisBus) RegistryList(ctx context.Context, ns string) (map[string][]byte, error) {
	if b.client == nil {
		return nil, ErrClosed
	}
	raw, err := b.client.HGetAll(ctx, b.hashKey(ns)).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(raw))
	for k, v := range raw {
		out[k] = []byte(v)
	}
	return out, nil
}

func (b *RedisBus) RegistryDelete(ctx context.Context, ns, key string) error {
	if b.client == nil {
		return ErrClosed
	}
	return b.client.HDel(ctx, b.hashKey(ns), key).Err()
}

func (b *RedisBus) QueuePush(ctx context.Context, name string, value []byte) error {
	if b.client == nil {
		return ErrClosed
	}
	// RPUSH + BLPOP = FIFO (append right, pop left).
	return b.client.RPush(ctx, b.queueKey(name), value).Err()
}

// QueuePop blocks (with short Redis-side timeouts) until a value is available
// or ctx is done.
func (b *RedisBus) QueuePop(ctx context.Context, name string) ([]byte, error) {
	if b.client == nil {
		return nil, ErrClosed
	}
	qkey := b.queueKey(name)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res, err := b.client.BLPop(ctx, 2*time.Second, qkey).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if len(res) >= 2 {
			return []byte(res[1]), nil
		}
	}
}

func (b *RedisBus) LogAppend(ctx context.Context, ns string, value []byte) (int64, error) {
	if b.client == nil {
		return -1, ErrClosed
	}
	n, err := b.client.RPush(ctx, b.logKey(ns), value).Result()
	if err != nil {
		return -1, err
	}
	return n - 1, nil // 0-based index
}

func (b *RedisBus) LogRead(ctx context.Context, ns string, cursor int64, limit int) ([][]byte, int64, error) {
	if b.client == nil {
		return nil, cursor, ErrClosed
	}
	if limit <= 0 {
		limit = 100
	}
	if cursor < 0 {
		cursor = 0
	}
	raw, err := b.client.LRange(ctx, b.logKey(ns), cursor, cursor+int64(limit)-1).Result()
	if err != nil {
		return nil, cursor, err
	}
	out := make([][]byte, len(raw))
	for i, v := range raw {
		out[i] = []byte(v)
	}
	return out, cursor + int64(len(out)), nil
}

var _ CoordBus = (*RedisBus)(nil)
