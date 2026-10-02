// tts/policy.go — the voice data policy (19.8): synthesized audio is
// PERSONALLY-IDENTIFIABLE content, so the defaults are conservative —
// DEFAULT: raw audio is never retained anywhere by this package (it is
// returned to the caller and dropped), usage records carry counters only
// (no text, no audio), and every retention/redaction knob is an explicit
// opt-in. Voice cost metrics (requests / characters / audio bytes per
// model) are counted by default — they contain no content.
package tts

import (
	"context"
	"sync"
)

// DataPolicy declares what a deployment does with synthesized content.
// The zero value is the safest policy: no audio retention, no text in
// usage records, counters only.
type DataPolicy struct {
	// RetainAudio opts INTO persisting synthesized audio: when true, Sink
	// receives every successful synthesis. Default false — raw audio is
	// never stored by this package.
	RetainAudio bool
	// Sink receives retained audio (only consulted when RetainAudio is
	// true). Wire your own store; nothing is written by default.
	Sink func(model string, text string, audio *Response)
	// RecordText opts INTO including the synthesized text in usage records
	// (auditing). Default false — usage records carry counters only.
	RecordText bool
	// OnUsage fires after every request (success or failure) with the
	// usage record; counters always, text only when RecordText is set.
	OnUsage func(Usage)
}

// Usage is one TTS usage record: cost metrics WITHOUT content by default.
// Characters counts runes of the synthesized text; AudioBytes the produced
// audio size (cost-relevant on every provider).
type Usage struct {
	Model      string
	Requests   int64 // includes failed requests
	Characters int64 // successful requests only
	AudioBytes int64
	// Text carries the synthesized text ONLY under RecordText (auditing
	// opt-in); empty under the default policy.
	Text string
}

// Meter counts TTS usage under a DataPolicy and mediates retention. Wrap
// any Model with Meter.Wrap; every Synthesize through the wrapper is
// counted, retained and reported per the policy.
type Meter struct {
	mu       sync.Mutex
	policy   DataPolicy
	perModel map[string]*Usage
}

// NewMeter builds a usage meter under the given policy.
func NewMeter(p DataPolicy) *Meter {
	return &Meter{policy: p, perModel: map[string]*Usage{}}
}

// Policy reports the meter's policy.
func (m *Meter) Policy() DataPolicy { return m.policy }

// Wrap decorates inner with usage counting and retention mediation.
func (m *Meter) Wrap(inner Model) Model {
	return &meteredModel{meter: m, inner: inner}
}

// Snapshot returns the accumulated usage records (sorted by model via
// Snapshot's caller; map order is not significant).
func (m *Meter) Snapshot() []Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Usage, 0, len(m.perModel))
	for _, u := range m.perModel {
		out = append(out, *u)
	}
	return out
}

// record books one request; called by meteredModel.
func (m *Meter) record(model, text string, audio *Response, failed bool) Usage {
	m.mu.Lock()
	u, ok := m.perModel[model]
	if !ok {
		u = &Usage{Model: model}
		m.perModel[model] = u
	}
	u.Requests++
	if !failed {
		u.Characters += int64(len([]rune(text)))
		if audio != nil {
			u.AudioBytes += int64(len(audio.Audio))
		}
	}
	rec := *u
	policy := m.policy
	m.mu.Unlock()

	// Content redaction: text reaches the record ONLY under the explicit
	// opt-in — usage records are content-free by default.
	if policy.RecordText && !failed {
		rec.Text = text
	}
	// Retention: audio is handed to the deployment's sink ONLY under the
	// explicit opt-in; never stored otherwise.
	if policy.RetainAudio && !failed && policy.Sink != nil && audio != nil {
		policy.Sink(model, text, audio)
	}
	if policy.OnUsage != nil {
		policy.OnUsage(rec)
	}
	return rec
}

// meteredModel is a Model wrapped by Meter.
type meteredModel struct {
	meter *Meter
	inner Model
}

func (w *meteredModel) ModelName() string { return w.inner.ModelName() }

func (w *meteredModel) Synthesize(ctx context.Context, text string, opts Options) (*Response, error) {
	resp, err := w.inner.Synthesize(ctx, text, opts)
	if err != nil {
		w.meter.record(w.inner.ModelName(), text, nil, true)
		return nil, err
	}
	w.meter.record(w.inner.ModelName(), text, resp, false)
	return resp, nil
}

var _ Model = (*meteredModel)(nil)
