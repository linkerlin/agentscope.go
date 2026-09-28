package gateway

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/linkerlin/agentscope.go/logging"
)

// RetentionLimits bounds the in-memory session state so long-lived replicas
// cannot leak (23.3).
type RetentionLimits struct {
	// CompletedMaxEntries caps the per-session completed replay buffers kept
	// in memory after a run finishes. Oldest-enqueued buffers are dropped
	// first. 0 disables the cap.
	CompletedMaxEntries int
	// CompletedTTL drops a completed buffer this long after its run ended.
	// 0 disables the TTL.
	CompletedTTL time.Duration
	// LogMaxEntries caps the coordinated event log per session (via the
	// bus's LogTrimmer). 0 disables trimming.
	LogMaxEntries int64
}

// DefaultRetentionLimits are the production defaults (23.3): completed
// buffers live at most one hour and at most 1024 sessions' worth; each
// session's coordinated event log keeps its newest 10k entries.
func DefaultRetentionLimits() RetentionLimits {
	return RetentionLimits{
		CompletedMaxEntries: 1024,
		CompletedTTL:        time.Hour,
		LogMaxEntries:       10_000,
	}
}

// RetentionStats reports what one retention pass dropped (the reaper's
// capacity metrics, 23.3).
type RetentionStats struct {
	// CompletedDropped is how many completed buffers were removed this pass.
	CompletedDropped int
	// CompletedLive is how many completed buffers remain.
	CompletedLive int
	// RegistryStaleDropped is how many expired running markers were removed
	// (coordination reaper only; -1 when the pass didn't check the registry).
	RegistryStaleDropped int
}

// enforceRetention drops expired and over-limit completed buffers. It is a
// pure function of the current time so tests can drive it deterministically.
func (sm *SessionManager) enforceRetention(now time.Time) RetentionStats {
	limits := sm.retention
	stats := RetentionStats{RegistryStaleDropped: -1}
	if limits.CompletedMaxEntries <= 0 && limits.CompletedTTL <= 0 {
		return stats
	}

	sm.mu.Lock()
	// TTL pass.
	if limits.CompletedTTL > 0 {
		for sid, at := range sm.completedAt {
			if now.Sub(at) > limits.CompletedTTL {
				delete(sm.completed, sid)
				delete(sm.completedAt, sid)
				stats.CompletedDropped++
			}
		}
	}
	// Cap pass: drop oldest-enqueued first.
	if limits.CompletedMaxEntries > 0 && len(sm.completed) > limits.CompletedMaxEntries {
		ids := make([]string, 0, len(sm.completedAt))
		for sid := range sm.completedAt {
			ids = append(ids, sid)
		}
		sort.Slice(ids, func(i, j int) bool {
			return sm.completedAt[ids[i]].Before(sm.completedAt[ids[j]])
		})
		for _, sid := range ids {
			if len(sm.completed) <= limits.CompletedMaxEntries {
				break
			}
			delete(sm.completed, sid)
			delete(sm.completedAt, sid)
			stats.CompletedDropped++
		}
	}
	stats.CompletedLive = len(sm.completed)
	sm.mu.Unlock()
	return stats
}

// EnforceRetention runs one retention pass and returns the drop stats.
func (sm *SessionManager) EnforceRetention() RetentionStats {
	return sm.enforceRetention(time.Now())
}

// SetRetention overrides the retention limits (nil-safe).
func (sm *SessionManager) SetRetention(l RetentionLimits) {
	sm.retention = l
}

// StartReaper launches the retention reaper: every interval it enforces the
// completed-buffer limits and — on the coordinator — CAS-deletes stale
// running markers from the registry, logging the capacity metrics (23.3).
// The returned stop func cancels the reaper and waits for its exit; it is
// idempotent.
func (c *SessionCoordinator) StartReaper(ctx context.Context, interval time.Duration) (stop func()) {
	if interval <= 0 {
		interval = time.Minute
	}
	reapCtx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-reapCtx.Done():
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := c.sm.EnforceRetention()
				stats.RegistryStaleDropped = c.dropStaleRunMarkers(reapCtx)
				log := logging.Default()
				log.Info("session retention pass",
					"completed_dropped", stats.CompletedDropped,
					"completed_live", stats.CompletedLive,
					"stale_markers_dropped", stats.RegistryStaleDropped,
				)
			}
		}
	}()
	var once bool
	return func() {
		if once {
			return
		}
		once = true
		ticker.Stop()
		cancel()
		<-done
	}
}

// dropStaleRunMarkers removes running-registry markers whose recorded lease
// expiry is in the past (a crashed replica's residue, 23.3). Deletion is
// CAS: a marker is only removed while it is exactly the stale one observed.
func (c *SessionCoordinator) dropStaleRunMarkers(ctx context.Context) int {
	if !c.coordinated() {
		return -1
	}
	entries, err := c.cb.RegistryList(ctx, c.keys.SessionRunRegistryNS())
	if err != nil || len(entries) == 0 {
		return 0
	}
	dropped := 0
	now := time.Now()
	for coordID, raw := range entries {
		var m leaseMarker
		if err := json.Unmarshal(raw, &m); err != nil || m.Expires.IsZero() {
			continue
		}
		if now.Before(m.Expires) {
			continue
		}
		if c.registryCAS != nil {
			if ok, _ := c.registryCAS.RegistryCompareAndDelete(ctx,
				c.keys.SessionRunRegistryNS(), coordID, raw); ok {
				dropped++
			}
			continue
		}
		if err := c.cb.RegistryDelete(ctx, c.keys.SessionRunRegistryNS(), coordID); err == nil {
			dropped++
		}
	}
	return dropped
}
