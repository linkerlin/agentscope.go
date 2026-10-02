package realtime

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/tts"
	"gopkg.in/yaml.v3"
)

// pcm16k is the canonical voice format used across the tests.
var pcm16k = AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}

// drain collects events until the channel closes or n events arrive.
func drain(ch <-chan Event, n int) []Event {
	var out []Event
	for ev := range ch {
		out = append(out, ev)
		if n > 0 && len(out) >= n {
			break
		}
	}
	return out
}

// TestContractEventSequence locks the session vocabulary and the stream
// ordering contract (19.1): open → deltas interleaved → terminal → channel
// closed; control-triggered steps fire in order and every call is recorded.
func TestContractEventSequence(t *testing.T) {
	card := ModelCard{Truncation: TruncationServer}
	card.ID = "mock-voice"
	card.Model = "mock-voice"
	script := []MockStep{
		// Auto: the assistant's opening turn, full vocabulary in one pass.
		{Auto: []Event{
			ResponseStarted{ResponseID: "r1"},
			AudioOutDelta{Sequence: 0, Data: []byte{1, 2}},
			TranscriptDelta{Text: "Hello"},
			AudioOutDelta{Sequence: 1, Data: []byte{3, 4}},
			ToolCall{ID: "tc1", Name: "lookup", Args: []byte(`{"q":"x"}`)},
			ResponseDone{ResponseID: "r1", Final: true},
		}},
		// Fires on the user's text input: barge-in then a cut response.
		{OnText: []Event{
			ResponseStarted{ResponseID: "r2"},
			AudioOutDelta{Sequence: 0, Data: []byte{9}},
			Interrupted{},
			ResponseDone{ResponseID: "r2", Final: false},
		}},
		// Fires on interrupt: clean close.
		{OnInterrupt: []Event{SessionClosed{}}},
	}
	m := NewMockModel(card, []AudioFormat{pcm16k}, script)
	sessI, answer, err := m.Connect(context.Background(), NegotiateOffer{Formats: []AudioFormat{pcm16k}})
	if err != nil {
		t.Fatal(err)
	}
	sess := sessI.(*MockSession) // mock-specific assertions below
	if !answer.Format.Equal(pcm16k) {
		t.Fatalf("negotiated format: %v", answer.Format)
	}

	// Turn 1: SessionStarted + the scripted turn (7 events + the open).
	got := drain(sess.Events(), 7)
	if len(got) != 7 {
		t.Fatalf("turn 1 events: got %d want 7 (%v)", len(got), got)
	}
	if st, ok := got[0].(SessionStarted); !ok || !st.Format.Equal(pcm16k) || st.SessionID == "" {
		t.Fatalf("first event must be SessionStarted with the negotiated format, got %#v", got[0])
	}
	if _, ok := got[1].(ResponseStarted); !ok {
		t.Fatalf("second event must be ResponseStarted, got %#v", got[1])
	}
	if d, ok := got[2].(AudioOutDelta); !ok || d.Sequence != 0 {
		t.Fatalf("audio delta sequence: %#v", got[2])
	}
	if _, ok := got[3].(TranscriptDelta); !ok {
		t.Fatalf("fourth event must be TranscriptDelta, got %#v", got[3])
	}
	if d, ok := got[4].(AudioOutDelta); !ok || d.Sequence != 1 {
		t.Fatalf("audio delta sequence 1: %#v", got[4])
	}
	if tc, ok := got[5].(ToolCall); !ok || tc.Name != "lookup" || string(tc.Args) != `{"q":"x"}` {
		t.Fatalf("tool call wrong: %#v", got[5])
	}
	if rd, ok := got[6].(ResponseDone); !ok || !rd.Final || rd.ResponseID != "r1" {
		t.Fatalf("turn must end with a final ResponseDone, got %#v", got[6])
	}

	// Turn 2: text input triggers the scripted barge-in turn.
	if err := sess.SendText(context.Background(), "stop that"); err != nil {
		t.Fatal(err)
	}
	got2 := drain(sess.Events(), 4)
	if len(got2) != 4 {
		t.Fatalf("turn 2 events: %v", got2)
	}
	if _, ok := got2[2].(Interrupted); !ok {
		t.Fatalf("turn 2 must carry Interrupted, got %v", got2)
	}
	if rd, ok := got2[3].(ResponseDone); !ok || rd.Final {
		t.Fatalf("interrupted turn must end Final=false, got %#v", got2[3])
	}

	// Interrupt → SessionClosed, then the channel closes (terminal).
	if err := sess.Interrupt(context.Background()); err != nil {
		t.Fatal(err)
	}
	got3 := drain(sess.Events(), 1)
	if len(got3) != 1 {
		t.Fatalf("expected SessionClosed, got %v", got3)
	}
	if _, ok := got3[0].(SessionClosed); !ok {
		t.Fatalf("expected SessionClosed, got %#v", got3[0])
	}
	if ev, open := <-sess.Events(); open {
		t.Fatalf("stream must be closed after the terminal event, got %#v", ev)
	}

	// Control calls recorded in order (a trailing "close" may appear when
	// the script exhausts and the mock self-closes — not an API call).
	calls := sess.Calls()
	if len(calls) < 2 || calls[0] != "text:stop that" || calls[1] != "interrupt" {
		t.Fatalf("call recording wrong: %v", calls)
	}
}

// TestContractSharedCardSchema: one YAML parses under BOTH tts.ModelCard
// and realtime.ModelCard — the shared-schema acceptance — with realtime
// fields additive and optional.
func TestContractSharedCardSchema(t *testing.T) {
	src := `
id: qwen-omni-realtime
provider: dashscope
display_name: Qwen Omni (Realtime)
model: qwen-omni-realtime
default_voice: Cherry
formats: [pcm]
realtime: true
multilingual: true
truncation: server
input_modalities: [audio, text]
audio_out:
  - codec: pcm
    sample_rate: 24000
    channels: 1
tools: true
`
	// Realtime card: every field lands.
	var rc ModelCard
	if err := yaml.Unmarshal([]byte(src), &rc); err != nil {
		t.Fatal(err)
	}
	if rc.ID != "qwen-omni-realtime" || rc.Provider != "dashscope" || !rc.Realtime || !rc.Tools {
		t.Fatalf("realtime card fields wrong: %+v", rc)
	}
	if rc.Truncation != TruncationServer {
		t.Fatalf("truncation: %q", rc.Truncation)
	}
	if len(rc.AudioOut) != 1 || rc.AudioOut[0].SampleRate != 24000 {
		t.Fatalf("audio_out: %+v", rc.AudioOut)
	}

	// The SAME yaml parses under the tts schema (shared core fields).
	var tc tts.ModelCard
	if err := yaml.Unmarshal([]byte(src), &tc); err != nil {
		t.Fatal(err)
	}
	if tc.ID != "qwen-omni-realtime" || tc.DefaultVoice != "Cherry" || !tc.Multilingual || !tc.Realtime {
		t.Fatalf("tts card fields wrong: %+v", tc)
	}

	// A plain TTS card (no realtime fields) is still a valid realtime card.
	plain := "id: tts-1\nprovider: openai\nmodel: tts-1\nformats: [pcm]\n"
	var rc2 ModelCard
	if err := yaml.Unmarshal([]byte(plain), &rc2); err != nil {
		t.Fatal(err)
	}
	if rc2.Truncation.Normalized() != DefaultTruncation {
		t.Fatalf("omitted truncation must normalize to the default, got %q", rc2.Truncation)
	}
}

// TestContractTruncationStates: the three declared states parse; unknown
// and empty values normalize to the conservative client-side cut.
func TestContractTruncationStates(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want TruncationSupport
	}{
		{"server", TruncationServer},
		{"client", TruncationClient},
		{"none", TruncationNone},
		{"", DefaultTruncation},
		{"typo", DefaultTruncation},
	} {
		var card ModelCard
		src := "id: x\nmodel: x\ntruncation: " + c.raw + "\n"
		if c.raw == "" {
			src = "id: x\nmodel: x\n"
		}
		if err := yaml.Unmarshal([]byte(src), &card); err != nil {
			t.Fatal(err)
		}
		if got := card.Truncation.Normalized(); got != c.want {
			t.Fatalf("truncation %q normalized to %q, want %q", c.raw, got, c.want)
		}
	}
}

// TestContractNegotiation: preference-ordered exact matching; a rate
// mismatch is a different format; no common format refuses; an empty
// server set accepts any offered format.
func TestContractNegotiation(t *testing.T) {
	pcm24k := AudioFormat{Codec: "pcm", SampleRate: 24000, Channels: 1}
	opus24k := AudioFormat{Codec: "opus", SampleRate: 24000, Channels: 1}
	supported := []AudioFormat{pcm16k, opus24k}

	// Client preference order wins: opus offered first → opus answered.
	ans, err := Negotiate(NegotiateOffer{Formats: []AudioFormat{opus24k, pcm24k}}, supported)
	if err != nil || !ans.Format.Equal(opus24k) {
		t.Fatalf("preference order: %+v err=%v", ans, err)
	}

	// Exact matching: pcm at the WRONG rate is not pcm — the first
	// preference is skipped and the second exact match wins.
	pcm48k := AudioFormat{Codec: "pcm", SampleRate: 48000, Channels: 1}
	ans1b, err := Negotiate(NegotiateOffer{Formats: []AudioFormat{pcm48k, pcm16k}}, supported)
	if err != nil || !ans1b.Format.Equal(pcm16k) {
		t.Fatalf("second exact preference must win: %+v err=%v", ans1b, err)
	}

	// No intersection refuses.
	if _, err := Negotiate(NegotiateOffer{Formats: []AudioFormat{pcm48k}}, supported); err != ErrNoCommonFormat {
		t.Fatalf("no common format must refuse, got %v", err)
	}

	// Empty server set: accept the client's top preference.
	ans2, err := Negotiate(NegotiateOffer{Formats: []AudioFormat{pcm48k, pcm16k}}, nil)
	if err != nil || !ans2.Format.Equal(pcm48k) {
		t.Fatalf("empty supported must accept top offer: %+v err=%v", ans2, err)
	}
}

// TestContractTerminal: which events end the SESSION stream (19.3
// correction): only SessionClosed/ErrorEvent. ResponseDone closes a turn,
// never the session — the conversation continues after a final response.
func TestContractTerminal(t *testing.T) {
	if Terminal(ResponseDone{Final: true}) || Terminal(ResponseDone{Final: false}) {
		t.Fatal("ResponseDone closes a turn, not the session")
	}
	if !Terminal(ErrorEvent{Err: "x"}) || !Terminal(SessionClosed{}) {
		t.Fatal("Error and SessionClosed are terminal")
	}
	if Terminal(AudioOutDelta{}) || Terminal(SessionStarted{}) {
		t.Fatal("deltas are not terminal")
	}
}
