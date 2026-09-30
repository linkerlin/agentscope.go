// gateway/worker.go realises the 18.5 long-running worker: the channel,
// wakeup and schedule loops are owned by role leases (CoordLease, 23.1) so
// they can run in a dedicated worker process instead of every API replica.
// Each role runs one acquire → hold (heartbeat) → lose → re-acquire loop:
//
//   - acquire: try to take the role lease; ErrLeaseHeld means another worker
//     holds it — back off exponentially and retry (standby replica, takes
//     over within one lease TTL after the holder dies);
//   - hold: start the role's component, renew the lease every TTL/3 (the
//     heartbeat doubles as reconciliation: a failed renewal means the fence
//     was superseded, so the component is stopped immediately);
//   - lose: stop the component and re-enter the acquire loop.
//
// Reconnect (bus hiccup, component start failure) rides the same exponential
// backoff. Runs are idempotent per role: Start/Stop pairs are supplied by the
// assembler (Server.Start) and must tolerate repeated invocation.
package gateway

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/logging"
	"github.com/linkerlin/agentscope.go/messagebus"
)

// WorkerRole names a long-running loop that can be split out of the API
// replicas (18.5).
type WorkerRole string

const (
	// RoleWakeup drains team inboxes and consumes persisted resume commands.
	RoleWakeup WorkerRole = "wakeup"
	// RoleSchedule executes cron-triggered agent turns.
	RoleSchedule WorkerRole = "schedule"
	// RoleChannel holds the outbound channel listeners (Discord/飞书/DingTalk
	// webhooks with stateful connections).
	RoleChannel WorkerRole = "channel"
)

// AllWorkerRoles is the default role set (single-process deployments).
var AllWorkerRoles = []WorkerRole{RoleWakeup, RoleSchedule, RoleChannel}

// WorkerConfig selects the long-running loops a process owns
// (AppConfig.Worker, 18.5). A nil WorkerConfig keeps the single-process
// default (all roles).
type WorkerConfig struct {
	// Roles lists the roles this process owns. nil inside a non-nil
	// WorkerConfig means all; an explicitly empty non-nil slice means none
	// (dedicated API replica — split deployments).
	Roles []WorkerRole
}

// RoleRunner binds a role to its component lifecycle. Start must be
// idempotent and non-blocking; Stop must terminate the component and be safe
// to call even when Start failed.
type RoleRunner struct {
	Role  WorkerRole
	Start func() error
	Stop  func()
}

// Default worker-lease tuning. The TTL is deliberately short: a crashed
// holder blocks a role for at most one TTL before a standby takes over.
const (
	DefaultWorkerLeaseTTL    = 15 * time.Second
	DefaultWorkerBackoffBase = 500 * time.Millisecond
	DefaultWorkerBackoffMax  = 30 * time.Second
)

// Worker owns role-guarded long-running components.
type Worker struct {
	lease   messagebus.CoordLease
	ownerID string
	ttl     time.Duration
	runners []RoleRunner

	log logging.Logger

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped sync.Once
}

// NewWorker creates a worker guarding the given roles with short-TTL leases
// on the given CoordLease-capable bus.
func NewWorker(lease messagebus.CoordLease, ownerID string, runners []RoleRunner) *Worker {
	return &Worker{
		lease:   lease,
		ownerID: ownerID,
		ttl:     DefaultWorkerLeaseTTL,
		runners: runners,
		log:     logging.Default(),
	}
}

// WithLeaseTTL overrides the role lease TTL (renewal follows at TTL/3);
// tests use it to exercise takeover quickly.
func (w *Worker) WithLeaseTTL(d time.Duration) *Worker {
	if d > 0 {
		w.ttl = d
	}
	return w
}

// Start launches one loop per role. Roles whose Start fails keep retrying
// with backoff; the worker itself never gives up until Stop.
func (w *Worker) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	for _, r := range w.runners {
		w.wg.Add(1)
		go w.runRole(ctx, r)
	}
}

// Stop tears down every role loop and waits for them to exit. Idempotent.
func (w *Worker) Stop() {
	w.stopped.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
	})
	w.wg.Wait()
}

// runRole is the acquire → hold → lose → re-acquire loop for one role.
func (w *Worker) runRole(ctx context.Context, r RoleRunner) {
	defer w.wg.Done()
	key := messagebus.Keys.WorkerRoleLockKey(string(r.Role))
	bo := newBackoff(DefaultWorkerBackoffBase, DefaultWorkerBackoffMax)

	for {
		if ctx.Err() != nil {
			return
		}
		// Acquire on a detached context: holding a role must not depend on
		// the lifecycle of any single request.
		l, release, err := w.lease.AcquireLease(context.Background(), key, w.ownerID, w.ttl)
		if err != nil {
			if !errors.Is(err, messagebus.ErrLeaseHeld) {
				w.log.Warn("worker: role lease acquire failed", "role", string(r.Role), "error", err.Error())
			}
			if !sleepCtx(ctx, bo.next()) {
				return
			}
			continue
		}
		bo.reset()

		if err := r.Start(); err != nil {
			w.log.Warn("worker: role start failed", "role", string(r.Role), "error", err.Error())
			release()
			if !sleepCtx(ctx, bo.next()) {
				return
			}
			continue
		}
		w.log.Info("worker: role acquired", "role", string(r.Role), "owner", w.ownerID)

		// Hold: heartbeat renews the lease; a failed renewal means the fence
		// was superseded (stalled past TTL, another worker took over).
		w.heartbeat(ctx, key, l, r)
		r.Stop()
		release()
		w.log.Info("worker: role released", "role", string(r.Role), "owner", w.ownerID)

		// Re-enter the acquire loop immediately: if another worker already
		// took over, the acquire fails fast and the backoff above applies.
	}
}

// heartbeat renews the role lease every ttl/3 until the context is done or a
// renewal fails (lease lost). It returns only after the role must stop.
func (w *Worker) heartbeat(ctx context.Context, key string, l *messagebus.Lease, r RoleRunner) {
	interval := w.ttl / 3
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// Renew once immediately so a lease that already expired between acquire
	// and start is detected right away instead of one interval later.
	if err := w.lease.RenewLease(ctx, key, l, w.ttl); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.lease.RenewLease(ctx, key, l, w.ttl); err != nil {
				w.log.Warn("worker: role lease lost", "role", string(r.Role), "error", err.Error())
				return
			}
		}
	}
}

// backoff is exponential with a ceiling; reset returns to the base.
type backoff struct {
	base, cur, max time.Duration
}

func newBackoff(base, max time.Duration) backoff {
	return backoff{base: base, cur: base, max: max}
}

func (b *backoff) next() time.Duration {
	d := b.cur
	b.cur *= 2
	if b.cur > b.max {
		b.cur = b.max
	}
	return d
}

func (b *backoff) reset() { b.cur = b.base }

// sleepCtx waits for d or ctx.Done; it reports whether the wait elapsed
// (false means the context is done and the caller should exit).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
