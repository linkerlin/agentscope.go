package messagebus

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"time"
)

// --- Fencing leases (23.1) ---

// Lease is a held fencing lease: exclusive access to a resource for ttl,
// guarded by a monotonically increasing Token. Holders must RenewLease
// before expiry; a holder that loses its lease (expiry + takeover by another
// owner) must stop making progress on the resource — the token is the fence
// that lets downstream state distinguish stale writes from live ones.
type Lease struct {
	Key     string
	Owner   string
	Token   uint64
	Expires time.Time
}

var (
	// ErrLeaseHeld is returned by AcquireLease when a live holder exists.
	ErrLeaseHeld = errors.New("messagebus: lease held by another owner")
	// ErrLeaseLost is returned by RenewLease when the lease is no longer the
	// current one (expired and taken over, or released): the caller must
	// stop its work (fence violated).
	ErrLeaseLost = errors.New("messagebus: lease lost (fence superseded)")
)

// CoordLease is the OPTIONAL fencing-lease extension of a bus (23.1). Unlike
// CoordBus.Lock (blocking mutex, TTL as crash protection only), a lease has
// a short TTL that MUST be renewed to keep running: a holder that dies
// (crash, network partition) is taken over after ttl, and every takeover
// bumps the fencing token so stale holders are detectable.
//
// Obtain via AsCoordLease; nil means the bus does not implement leases and
// callers should fall back to CoordBus.Lock.
type CoordLease interface {
	// AcquireLease acquires key for owner. Non-blocking: when a live holder
	// exists it returns ErrLeaseHeld. The returned Cancel releases the lease
	// (token-gated: a release after losing the lease is a no-op).
	AcquireLease(ctx context.Context, key, owner string, ttl time.Duration) (*Lease, Cancel, error)
	// RenewLease extends the lease to now+ttl when — and only when — key
	// still holds exactly this lease (token CAS). Any other state (expired,
	// taken over, released) returns ErrLeaseLost.
	RenewLease(ctx context.Context, key string, l *Lease, ttl time.Duration) error
}

// RegistryCAS is the OPTIONAL compare-and-delete extension of CoordBus's
// registry (23.1): cleanup of registry entries must only remove what the
// caller wrote, never a successor's fresher entry.
type RegistryCAS interface {
	// RegistryCompareAndDelete deletes ns/key only when its current value
	// equals expect. Returns whether it deleted.
	RegistryCompareAndDelete(ctx context.Context, ns, key string, expect []byte) (bool, error)
}

// AsCoordLease returns a CoordLease view of b if it implements leases, else nil.
func AsCoordLease(b Bus) CoordLease {
	if cl, ok := b.(CoordLease); ok {
		return cl
	}
	return nil
}

// AsRegistryCAS returns a RegistryCAS view of b if it implements CAS
// registry deletion, else nil.
func AsRegistryCAS(b Bus) RegistryCAS {
	if rc, ok := b.(RegistryCAS); ok {
		return rc
	}
	return nil
}

// leaseValue is the wire format stored under a lease key: "owner|token".
func leaseValue(l *Lease) string { return l.Owner + "|" + strconv.FormatUint(l.Token, 10) }

// --- LocalBus implementation ---

// localLease is the in-process lease record.
type localLease struct {
	lease   Lease
	expires time.Time
	timer   *time.Timer
}

// AcquireLease implements CoordLease. The per-key fencing counter is a
// process-wide atomic: tokens stay monotonic across lease-map churn.
func (b *LocalBus) AcquireLease(ctx context.Context, key, owner string, ttl time.Duration) (*Lease, Cancel, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	fenceAny, _ := b.fences.LoadOrStore(key, &atomic.Uint64{})
	fence := fenceAny.(*atomic.Uint64)
	token := fence.Add(1)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, ErrClosed
	}
	if cur, ok := b.leases[key]; ok && time.Now().Before(cur.expires) {
		return nil, nil, ErrLeaseHeld
	}
	// Fresh (or expired) lease: take over with a strictly greater token.
	l := &Lease{Key: key, Owner: owner, Token: token, Expires: time.Now().Add(ttl)}
	rec := &localLease{lease: *l, expires: l.Expires}
	if ttl > 0 {
		// Expiry reaper: delete the record only when it still holds this
		// token (CAS), so a renewed lease is never removed by a stale timer.
		rec.timer = time.AfterFunc(ttl, func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if cur, ok := b.leases[key]; ok && cur.lease.Token == token {
				delete(b.leases, key)
			}
		})
	}
	b.leases[key] = rec

	var once bool
	cancel := Cancel(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if once {
			return
		}
		once = true
		if rec.timer != nil {
			rec.timer.Stop()
		}
		if cur, ok := b.leases[key]; ok && cur.lease.Token == token {
			delete(b.leases, key)
		}
	})
	out := *l
	return &out, cancel, nil
}

// RenewLease implements CoordLease (token CAS).
func (b *LocalBus) RenewLease(ctx context.Context, key string, l *Lease, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	cur, ok := b.leases[key]
	if !ok || cur.lease.Token != l.Token || cur.lease.Owner != l.Owner || !time.Now().Before(cur.expires) {
		return ErrLeaseLost
	}
	cur.expires = time.Now().Add(ttl)
	if cur.timer != nil {
		cur.timer.Reset(ttl)
	}
	l.Expires = cur.expires
	return nil
}

// RegistryCompareAndDelete implements RegistryCAS.
func (b *LocalBus) RegistryCompareAndDelete(ctx context.Context, ns, key string, expect []byte) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false, ErrClosed
	}
	m, ok := b.registry[ns]
	if !ok {
		return false, nil
	}
	if string(m[key]) != string(expect) {
		return false, nil
	}
	delete(m, key)
	if len(m) == 0 {
		delete(b.registry, ns)
	}
	return true, nil
}

// --- RedisBus implementation ---

// leaseRenewScript extends the lease TTL only when the stored value still
// equals this holder's "owner|token" (CAS). Returns 1 on success, 0 when the
// fence is superseded.
const leaseRenewScript = `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('pexpire', KEYS[1], ARGV[2]) else return 0 end`

// leaseAcquireScript takes the lease only when the key is absent, bumping
// the per-key fencing counter first so tokens are monotonic across owners.
// owner prefix and token are concatenated inside the script so the counter
// increment and the SET NX PX are one atomic step. Returns the token on
// success, nil when held.
const leaseAcquireScript = `local t = redis.call('incr', KEYS[2]) if redis.call('set', KEYS[1], ARGV[1] .. t, 'nx', 'px', ARGV[2]) then return t else return nil end`

func (b *RedisBus) leaseKey(key string) string { return b.prefix + ":lease:" + key }
func (b *RedisBus) fenceKey(key string) string { return b.prefix + ":fence:" + key }

// AcquireLease implements CoordLease over Redis. The Lua script makes
// counter-increment and SET NX PX atomic: when the key is held the counter
// still advances (monotonicity) but no lease is taken (ErrLeaseHeld); when
// the key is absent (fresh or expired — Redis expiry is authoritative) the
// caller takes over with a strictly greater token.
func (b *RedisBus) AcquireLease(ctx context.Context, key, owner string, ttl time.Duration) (*Lease, Cancel, error) {
	if b.client == nil {
		return nil, nil, ErrClosed
	}
	px := strconv.FormatInt(ttl.Milliseconds(), 10)
	prefix := owner + "|"
	res, err := b.client.Eval(ctx, leaseAcquireScript, []string{b.leaseKey(key), b.fenceKey(key)},
		prefix, px).Result()
	if err != nil {
		return nil, nil, err
	}
	if res == nil {
		return nil, nil, ErrLeaseHeld
	}
	var token uint64
	switch v := res.(type) {
	case int64:
		token = uint64(v)
	case string:
		token, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, errors.New("messagebus: unexpected lease token type from redis")
	}
	l := &Lease{Key: key, Owner: owner, Token: token, Expires: time.Now().Add(ttl)}
	full := leaseValue(l)

	var once bool
	cancel := Cancel(func() {
		if once {
			return
		}
		once = true
		// token-gated DEL: a stale holder's release cannot evict a successor.
		_ = b.client.Eval(context.Background(), lockReleaseScript, []string{b.leaseKey(key)}, full).Err()
	})
	out := *l
	return &out, cancel, nil
}

// RenewLease implements CoordLease (Lua token CAS).
func (b *RedisBus) RenewLease(ctx context.Context, key string, l *Lease, ttl time.Duration) error {
	if b.client == nil {
		return ErrClosed
	}
	ok, err := b.client.Eval(ctx, leaseRenewScript, []string{b.leaseKey(key)},
		leaseValue(l), strconv.FormatInt(ttl.Milliseconds(), 10)).Result()
	if err != nil {
		return err
	}
	if ok == int64(0) {
		return ErrLeaseLost
	}
	l.Expires = time.Now().Add(ttl)
	return nil
}

// RegistryCompareAndDelete implements RegistryCAS (Lua value CAS on the hash).
const registryCADScript = `if redis.call('hget', KEYS[1], KEYS[2]) == ARGV[1] then redis.call('hdel', KEYS[1], KEYS[2]) return 1 else return 0 end`

func (b *RedisBus) RegistryCompareAndDelete(ctx context.Context, ns, key string, expect []byte) (bool, error) {
	if b.client == nil {
		return false, ErrClosed
	}
	n, err := b.client.Eval(ctx, registryCADScript, []string{b.hashKey(ns), key}, string(expect)).Result()
	if err != nil {
		return false, err
	}
	return n == int64(1), nil
}

var (
	_ CoordLease  = (*LocalBus)(nil)
	_ RegistryCAS = (*LocalBus)(nil)
	_ CoordLease  = (*RedisBus)(nil)
	_ RegistryCAS = (*RedisBus)(nil)
)
