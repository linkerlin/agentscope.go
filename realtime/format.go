// realtime/format.go — audio format negotiation (19.1): the client offers
// an ordered preference list, the server answers with one binding format.
// Negotiation is pure so every backend and the agent share one definition.
package realtime

import (
	"fmt"
	"sort"
)

// AudioFormat is one concrete audio encoding. Codec is a lowercase
// short name: "pcm" (raw little-endian 16-bit), "opus", "g711_ulaw", …
type AudioFormat struct {
	Codec      string `yaml:"codec" json:"codec"`
	SampleRate int    `yaml:"sample_rate" json:"sample_rate"` // Hz, e.g. 16000, 24000
	Channels   int    `yaml:"channels" json:"channels"`       // 1 = mono
}

// String renders the format for logs ("pcm@24k/1").
func (f AudioFormat) String() string {
	return fmt.Sprintf("%s@%dk/%d", f.Codec, f.SampleRate/1000, f.Channels)
}

// Equal reports exact-format equality (codec AND rate AND channels — a
// sample-rate mismatch is a different format).
func (f AudioFormat) Equal(o AudioFormat) bool {
	return f.Codec == o.Codec && f.SampleRate == o.SampleRate && f.Channels == o.Channels
}

// ErrNoCommonFormat is returned when the offer and the server's supported
// set have no exact match: the session cannot start (negotiation must not
// silently transcode — that would break latency budgets).
var ErrNoCommonFormat = fmt.Errorf("realtime: no common audio format")

// Negotiate picks the best format for one session: the client's most
// preferred format that the server also lists. supported is the server's
// unordered set; an empty supported set means "accept anything the client
// offered" (the backend adapts itself — e.g. gateway-side resampling
// declared up front).
func Negotiate(offer NegotiateOffer, supported []AudioFormat) (NegotiateAnswer, error) {
	for _, want := range offer.Formats {
		if len(supported) == 0 {
			return NegotiateAnswer{Format: want}, nil
		}
		for _, have := range supported {
			if want.Equal(have) {
				return NegotiateAnswer{Format: want}, nil
			}
		}
	}
	if len(supported) == 0 && len(offer.Formats) > 0 {
		// Unreachable (handled above); kept for clarity.
		return NegotiateAnswer{Format: offer.Formats[0]}, nil
	}
	return NegotiateAnswer{}, ErrNoCommonFormat
}

// SortFormatsByPreference orders formats codec-first then descending rate
// (helper for building deterministic offers in tests and clients).
func SortFormatsByPreference(fs []AudioFormat) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Codec != fs[j].Codec {
			return fs[i].Codec < fs[j].Codec
		}
		if fs[i].SampleRate != fs[j].SampleRate {
			return fs[i].SampleRate > fs[j].SampleRate
		}
		return fs[i].Channels < fs[j].Channels
	})
}
