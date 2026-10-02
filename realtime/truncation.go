// realtime/truncation.go — barge-in truncation capability (19.1 三态):
// who cuts the queued audio when the user interrupts.
package realtime

// TruncationSupport declares how a model handles cutting playback on
// barge-in. The three meaningful states:
type TruncationSupport string

const (
	// TruncationServer: the backend cuts its own output queue on an
	// Interrupt — the client only stops playing what it receives.
	TruncationServer TruncationSupport = "server"
	// TruncationClient: the backend keeps streaming; the CLIENT must cut at
	// the playout position confirmed by playback (19.2's PlayoutPosition).
	TruncationClient TruncationSupport = "client"
	// TruncationNone: no truncation — an interrupt only stops future
	// generation, already-queued audio plays out.
	TruncationNone TruncationSupport = "none"
)

// DefaultTruncation applies when a card omits the field: the conservative
// client-side cut (the agent can always stop local playback; assuming
// server-side truncation that does not exist would leave stale audio).
const DefaultTruncation = TruncationClient

// Normalized returns the card's declared support, or the conservative
// default when empty/unknown. Unknown values normalize to the default too
// (a typo'd card must not silently disable truncation).
func (t TruncationSupport) Normalized() TruncationSupport {
	switch t {
	case TruncationServer, TruncationClient, TruncationNone:
		return t
	}
	return DefaultTruncation
}
