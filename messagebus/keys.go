package messagebus

// This file centralises business-specific message-bus key/namespace conventions
// so the Bus itself stays domain-agnostic (it exposes only generic primitives).
// Mirrors Python agentscope's app/message_bus/_keys.py (MessageBusKeys).
//
// Add new business keys here as needed.

// WakeupKind discriminates how the WakeupDispatcher should spawn a run when
// draining the shared trigger queue.
type WakeupKind string

const (
	// WakeupKindWake wakes an idle session to drain pending inbox content. The
	// dispatcher spawns the run with an empty input and skips the session while
	// it is already running.
	WakeupKindWake WakeupKind = "wake"
	// WakeupKindResume feeds a parked session an awaiting tool-call result
	// (human-in-the-loop). Unlike wake, the dispatcher must not drop the entry
	// while the session is running; it re-queues until the parked run releases.
	WakeupKindResume WakeupKind = "resume"
)

// CoordKeys holds business-layer key/namespace helpers built on top of the
// generic Bus/CoordBus primitives. Use these instead of raw strings so the key
// format is auditable in one place.
type CoordKeys struct{}

// QueueName returns the bus queue name for a logical queue.
func (CoordKeys) QueueName(logical string) string { return "as:queue:" + logical }

// LockKey returns the lock key for a logical resource.
func (CoordKeys) LockKey(resource string) string { return "as:lock:" + resource }

// RegistryNS returns the registry namespace for a logical group.
func (CoordKeys) RegistryNS(group string) string { return "as:reg:" + group }

// LogNS returns the log namespace for a logical log stream.
func (CoordKeys) LogNS(name string) string { return "as:log:" + name }

// ProjectionNS returns the registry namespace for a session's cross-session UI
// projections (e.g. a worker's HITL request projected onto its leader session).
func (CoordKeys) ProjectionNS(targetSessionID string) string {
	return "as:projection:" + targetSessionID
}

// --- Session coordination (18.1) ---

// SessionRunLockKey returns the distributed lock key guarding a session's
// running turn: at most one replica executes a session at a time.
func (CoordKeys) SessionRunLockKey(sessionID string) string {
	return CoordKeys{}.LockKey("session-run:" + sessionID)
}

// SessionRunRegistryNS returns the registry namespace holding one entry per
// currently-running session (value: coordinator metadata JSON).
func (CoordKeys) SessionRunRegistryNS() string {
	return CoordKeys{}.RegistryNS("session-run")
}

// SessionEventLogNS returns the append-log namespace for a session's turn
// events, enabling replay from any replica.
func (CoordKeys) SessionEventLogNS(sessionID string) string {
	return CoordKeys{}.LogNS("session-events:" + sessionID)
}

// SessionCancelChannel returns the bus pub/sub channel used to request a
// running session's cancellation from any replica.
func (CoordKeys) SessionCancelChannel(sessionID string) string {
	return "as:cancel:session:" + sessionID
}

// Default CoordKeys instance for convenience.
var Keys = CoordKeys{}
