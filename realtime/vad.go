// realtime/vad.go — voice activity detection with correct EDGES (19.2):
// an energy detector over 16-bit PCM with hangover debounce. The detector
// never emits noise: exactly one SpeechStart when sustained speech is
// confirmed, exactly one SpeechEnd when speech is confirmed over.
package realtime

import "math"

// VADEvent is an edge, not a level: the detector reports transitions only.
type VADEvent string

const (
	// VADNone: this frame changed nothing.
	VADNone VADEvent = ""
	// VADSpeechStart: speech confirmed (MinSpeechFrames above threshold).
	VADSpeechStart VADEvent = "speech_start"
	// VADSpeechEnd: silence confirmed (HangoverFrames below threshold).
	VADSpeechEnd VADEvent = "speech_end"
)

// VADConfig tunes the detector. Sensible voice defaults for 16-bit PCM at
// 16kHz with 20ms frames (320 samples): Threshold ~350 RMS,
// MinSpeechFrames 3 (60ms), HangoverFrames 10 (200ms).
type VADConfig struct {
	// Threshold is the RMS level above which a frame counts as speech.
	Threshold float64
	// MinSpeechFrames consecutive speech frames confirm a start (filters
	// clicks).
	MinSpeechFrames int
	// HangoverFrames consecutive silent frames confirm an end (bridges
	// intra-word pauses).
	HangoverFrames int
}

// DefaultVADConfig returns the voice-tuned defaults above.
func DefaultVADConfig() VADConfig {
	return VADConfig{Threshold: 350, MinSpeechFrames: 3, HangoverFrames: 10}
}

// VAD is the streaming detector. Feed Process one PCM frame at a time; it
// returns the edge (if any) for that frame.
type VAD struct {
	cfg        VADConfig
	speaking   bool
	speechRun  int
	silenceRun int
	lastLevel  float64
}

// NewVAD builds a detector (idle at start).
func NewVAD(cfg VADConfig) *VAD {
	if cfg.MinSpeechFrames <= 0 {
		cfg.MinSpeechFrames = 3
	}
	if cfg.HangoverFrames <= 0 {
		cfg.HangoverFrames = 10
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 350
	}
	return &VAD{cfg: cfg}
}

// Process consumes one frame of 16-bit little-endian-candidate PCM (native
// []int16) and reports the edge it produced.
func (v *VAD) Process(pcm []int16) VADEvent {
	level := rms(pcm)
	v.lastLevel = level
	isSpeech := level >= v.cfg.Threshold

	if !v.speaking {
		if isSpeech {
			v.speechRun++
			if v.speechRun >= v.cfg.MinSpeechFrames {
				v.speaking = true
				v.speechRun = 0
				v.silenceRun = 0
				return VADSpeechStart
			}
		} else {
			v.speechRun = 0
		}
		return VADNone
	}

	// Speaking: hangover before ending.
	if isSpeech {
		v.silenceRun = 0
		return VADNone
	}
	v.silenceRun++
	if v.silenceRun >= v.cfg.HangoverFrames {
		v.speaking = false
		v.silenceRun = 0
		v.speechRun = 0
		return VADSpeechEnd
	}
	return VADNone
}

// Speaking reports the current level (for observability, not edge logic).
func (v *VAD) Speaking() bool { return v.speaking }

// LastLevel reports the most recent frame RMS.
func (v *VAD) LastLevel() float64 { return v.lastLevel }

// rms computes the root-mean-square level of a PCM frame.
func rms(pcm []int16) float64 {
	if len(pcm) == 0 {
		return 0
	}
	var sum float64
	for _, s := range pcm {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(pcm)))
}

// ToneSynth generates one frame of a sine tone at the given amplitude —
// the test/driver helper for deterministic VAD input.
func ToneSynth(samples int, amplitude int16) []int16 {
	out := make([]int16, samples)
	for i := range out {
		out[i] = int16(float64(amplitude) * math.Sin(2*math.Pi*float64(i)/32))
	}
	return out
}

// SilenceSynth generates one frame of silence.
func SilenceSynth(samples int) []int16 { return make([]int16, samples) }
