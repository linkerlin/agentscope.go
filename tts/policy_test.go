// tts/policy_test.go — the 19.8 acceptance tests: Gemini TTS payload /
// error / card contract, CosyVoice v3 and gpt-4o-mini-tts payload+card
// contracts, and the voice data policy (default no-retention, explicit
// retention/redaction opt-ins, content-free cost metrics).
package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
)

// ---- Gemini TTS ----

// TestGeminiTTS_PayloadAndResponse locks the wire contract: model in the
// URL path, key in the query, AUDIO response modality + prebuilt voice in
// the payload, inline PCM audio decoded back with the MIME stripped of
// ";rate=...".
func TestGeminiTTS_PayloadAndResponse(t *testing.T) {
	var gotPath, gotQuery string
	var gotBody geminiTTSRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		audio := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})
		fmt.Fprintf(w, `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/pcm;rate=24000","data":%q}}]}}]}`, audio)
	}))
	defer srv.Close()

	g := NewGemini("g-key").WithBaseURL(srv.URL)
	resp, err := g.Synthesize(context.Background(), "你好", Options{Voice: "Puck"})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if !strings.Contains(gotPath, "models/gemini-2.5-flash-preview-tts:generateContent") {
		t.Fatalf("url path: %s", gotPath)
	}
	if gotQuery != "key=g-key" {
		t.Fatalf("query: %s", gotQuery)
	}
	if len(gotBody.Contents) != 1 || gotBody.Contents[0].Parts[0].Text != "你好" {
		t.Fatalf("contents: %+v", gotBody.Contents)
	}
	if len(gotBody.GenerationConfig.ResponseModalities) != 1 || gotBody.GenerationConfig.ResponseModalities[0] != "AUDIO" {
		t.Fatalf("modalities: %v", gotBody.GenerationConfig.ResponseModalities)
	}
	if gotBody.GenerationConfig.SpeechConfig == nil ||
		gotBody.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName != "Puck" {
		t.Fatalf("voice config: %+v", gotBody.GenerationConfig.SpeechConfig)
	}
	if string(resp.Audio) != "\x01\x02\x03\x04" || resp.MediaType != "audio/pcm" || !resp.IsLast {
		t.Fatalf("response: %+v", resp)
	}
}

// TestGeminiTTS_Error surfaces the server's error message.
func TestGeminiTTS_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"voice not supported"}}`))
	}))
	defer srv.Close()

	g := NewGemini("g-key").WithBaseURL(srv.URL)
	_, err := g.Synthesize(context.Background(), "hi", Options{})
	if err == nil {
		t.Fatal("must fail")
	}
	if !strings.Contains(err.Error(), "voice not supported") || !strings.Contains(err.Error(), "INVALID_ARGUMENT") {
		t.Fatalf("error: %v", err)
	}
}

// TestGeminiTTS_Cards: both Gemini TTS cards load.
func TestGeminiTTS_Cards(t *testing.T) {
	cards, err := ListModelCards()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ModelCard{}
	for _, c := range cards {
		byID[c.ID] = *c
	}
	for _, id := range []string{"gemini-2.5-flash-preview-tts", "gemini-2.5-pro-tts"} {
		c, ok := byID[id]
		if !ok {
			t.Fatalf("missing card %s", id)
		}
		if c.Provider != "google" || c.DefaultVoice != "Kore" {
			t.Fatalf("card %s: %+v", id, c)
		}
	}
}

// ---- CosyVoice v3 (DashScope backend, model-parameterised) ----

// TestDashScope_CosyVoiceV3 locks the v3 payload: model name and voice on
// the wire, audio decoded, plus the embedded card.
func TestDashScope_CosyVoiceV3(t *testing.T) {
	var gotBody dashscopeTTSRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		audio := base64.StdEncoding.EncodeToString([]byte{9, 9})
		fmt.Fprintf(w, `{"output":{"audio":%q,"request_id":"r1"}}`, audio)
	}))
	defer srv.Close()

	d := NewDashScope("d-key").WithBaseURL(srv.URL).WithModel("cosyvoice-v3").WithVoice("longxiaochun_v2")
	resp, err := d.Synthesize(context.Background(), "你好", Options{})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if gotBody.Model != "cosyvoice-v3" || gotBody.Parameters.Voice != "longxiaochun_v2" || gotBody.Input.Text != "你好" {
		t.Fatalf("payload: %+v", gotBody)
	}
	if string(resp.Audio) != "\x09\x09" || resp.MediaType != "audio/mpeg" {
		t.Fatalf("response: %+v", resp)
	}

	cards, _ := ListModelCards()
	found := false
	for _, c := range cards {
		if c.Model == "cosyvoice-v3" && c.DefaultVoice == "longxiaochun_v2" {
			found = true
		}
	}
	if !found {
		t.Fatal("cosyvoice-v3 card missing")
	}
}

// ---- gpt-4o-mini-tts (OpenAI adapter) ----

type fakeSynthesizer struct {
	model   string
	gotOpts model.AudioOptions
	gotText string
	err     error
}

func (f *fakeSynthesizer) SynthesizeSpeech(ctx context.Context, text string, opts model.AudioOptions) ([]byte, error) {
	f.gotText, f.gotOpts = text, opts
	if f.err != nil {
		return nil, f.err
	}
	return []byte("audio-bytes"), nil
}

func (f *fakeSynthesizer) ModelName() string { return f.model }

// fakeModel is a full tts.Model fake (Meter tests).
type fakeModel struct {
	model string
	err   error
}

func (f *fakeModel) ModelName() string { return f.model }

func (f *fakeModel) Synthesize(ctx context.Context, text string, opts Options) (*Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &Response{Audio: []byte("audio-bytes"), MediaType: "audio/mpeg", IsLast: true}, nil
}

// TestOpenAIAdapter_GPT4oMiniTTS: the adapter passes voice/format/speed
// through, wraps backend errors, and the model card loads.
func TestOpenAIAdapter_GPT4oMiniTTS(t *testing.T) {
	fake := &fakeSynthesizer{model: "gpt-4o-mini-tts"}
	adapter := NewSynthesizerAdapter(fake)

	if adapter.ModelName() != "gpt-4o-mini-tts" {
		t.Fatalf("model name: %s", adapter.ModelName())
	}
	resp, err := adapter.Synthesize(context.Background(), "你好", Options{Voice: "coral", Format: "flac", Speed: 1.2})
	if err != nil {
		t.Fatal(err)
	}
	if fake.gotText != "你好" || fake.gotOpts.Voice != "coral" || fake.gotOpts.Format != "flac" || fake.gotOpts.Speed != 1.2 {
		t.Fatalf("passed options: %+v text=%q", fake.gotOpts, fake.gotText)
	}
	if string(resp.Audio) != "audio-bytes" || resp.MediaType != "audio/flac" {
		t.Fatalf("response: %+v", resp)
	}

	// Error contract: backend failures surface as "tts openai: ...".
	failing := &fakeSynthesizer{model: "gpt-4o-mini-tts", err: errors.New("quota exceeded")}
	_, err = NewSynthesizerAdapter(failing).Synthesize(context.Background(), "x", Options{})
	if err == nil || !strings.Contains(err.Error(), "tts openai") || !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("error wrap: %v", err)
	}

	cards, _ := ListModelCards()
	found := false
	for _, c := range cards {
		if c.Model == "gpt-4o-mini-tts" && c.Provider == "openai" {
			found = true
		}
	}
	if !found {
		t.Fatal("gpt-4o-mini-tts card missing")
	}
}

// ---- voice data policy ----

// TestMeter_DefaultNoRetention: the zero-value policy counts usage WITHOUT
// content — no audio reaches any sink, usage records carry counters only.
func TestMeter_DefaultNoRetention(t *testing.T) {
	var sinkCalls int
	fake := &fakeModel{model: "m1"}
	meter := NewMeter(DataPolicy{
		// Deliberately set a sink WITHOUT RetainAudio: the default must not
		// call it.
		Sink: func(string, string, *Response) { sinkCalls++ },
	})
	var mu sync.Mutex
	var records []Usage
	meter.policy.OnUsage = func(u Usage) {
		mu.Lock()
		records = append(records, u)
		mu.Unlock()
	}

	wrapped := meter.Wrap(fake)
	if _, err := wrapped.Synthesize(context.Background(), "你好世界", Options{}); err != nil {
		t.Fatal(err)
	}

	if sinkCalls != 0 {
		t.Fatalf("default policy must NOT retain audio, sink called %d times", sinkCalls)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(records) != 1 {
		t.Fatalf("usage records: %d", len(records))
	}
	u := records[0]
	if u.Model != "m1" || u.Requests != 1 || u.Characters != 4 || u.AudioBytes != int64(len("audio-bytes")) {
		t.Fatalf("usage: %+v", u)
	}
	if u.Text != "" {
		t.Fatalf("usage record must be content-free by default: %+v", u)
	}
}

// TestMeter_ExplicitRetentionAndText: retention and text auditing are
// explicit opt-ins — with both set, the sink receives the audio and usage
// records carry the text.
func TestMeter_ExplicitRetentionAndText(t *testing.T) {
	fake := &fakeModel{model: "m2"}
	var sinkModel, sinkText string
	var sinkAudio []byte
	var lastUsage Usage
	meter := NewMeter(DataPolicy{
		RetainAudio: true,
		Sink:        func(m, text string, a *Response) { sinkModel, sinkText, sinkAudio = m, text, a.Audio },
		RecordText:  true,
		OnUsage:     func(u Usage) { lastUsage = u },
	})
	if _, err := meter.Wrap(fake).Synthesize(context.Background(), "文本", Options{}); err != nil {
		t.Fatal(err)
	}
	if sinkModel != "m2" || sinkText != "文本" || string(sinkAudio) != "audio-bytes" {
		t.Fatalf("sink: %s %q %q", sinkModel, sinkText, sinkAudio)
	}
	// The per-request usage record carries the text (auditing opt-in); the
	// accumulated Snapshot stays counter-only by design (multiple requests
	// never concatenate their texts).
	if lastUsage.Text != "文本" {
		t.Fatalf("RecordText must surface text on OnUsage: %+v", lastUsage)
	}
	if u := meter.Snapshot()[0]; u.Text != "" {
		t.Fatalf("Snapshot is cumulative counters, never text: %+v", u)
	}
}

// TestMeter_FailedRequests: failures count as requests (cost observability)
// but add no characters and never reach the retention sink.
func TestMeter_FailedRequests(t *testing.T) {
	fake := &fakeModel{model: "m3", err: errors.New("boom")}
	var sinkCalls int
	meter := NewMeter(DataPolicy{
		RetainAudio: true,
		Sink:        func(string, string, *Response) { sinkCalls++ },
	})
	if _, err := meter.Wrap(fake).Synthesize(context.Background(), "你好", Options{}); err == nil {
		t.Fatal("must fail")
	}
	if sinkCalls != 0 {
		t.Fatalf("failed synthesis must not retain: %d", sinkCalls)
	}
	snap := meter.Snapshot()
	if len(snap) != 1 || snap[0].Requests != 1 || snap[0].Characters != 0 || snap[0].AudioBytes != 0 {
		t.Fatalf("usage: %+v", snap)
	}
}

// TestMeter_ConcurrentCounts: counters are race-safe under concurrent
// synthesis.
func TestMeter_ConcurrentCounts(t *testing.T) {
	fake := &fakeModel{model: "m4"}
	meter := NewMeter(DataPolicy{})
	wrapped := meter.Wrap(fake)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = wrapped.Synthesize(context.Background(), "ab", Options{})
		}()
	}
	wg.Wait()
	snap := meter.Snapshot()
	if snap[0].Requests != 20 || snap[0].Characters != 40 {
		t.Fatalf("usage: %+v", snap)
	}
}
