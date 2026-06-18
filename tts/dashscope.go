package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DashScope implements a non-realtime tts.Model backed by Alibaba Cloud
// DashScope text-to-speech (CosyVoice / qwen3-tts), using the synchronous
// generation API. It mirrors Python agentscope's tts/_dashscope backend (#1832).
type DashScope struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string // cosyvoice-v1 | qwen3-tts-flash | ...
	voice      string // default voice, e.g. "longxiaochun"
	format     string // default format, e.g. "mp3"
}

// NewDashScope creates a DashScope TTS model with sensible defaults.
func NewDashScope(apiKey string) *DashScope {
	return &DashScope{
		apiKey:     apiKey,
		baseURL:    "https://dashscope.aliyuncs.com",
		httpClient: &http.Client{Timeout: 60 * time.Second},
		model:      "cosyvoice-v1",
		voice:      "longxiaochun",
		format:     "mp3",
	}
}

// WithBaseURL overrides the API base URL (e.g. for a proxy or test server).
func (d *DashScope) WithBaseURL(url string) *DashScope {
	d.baseURL = url
	return d
}

// WithModel sets the TTS model name.
func (d *DashScope) WithModel(m string) *DashScope {
	d.model = m
	return d
}

// WithVoice sets the default speaker voice.
func (d *DashScope) WithVoice(v string) *DashScope {
	d.voice = v
	return d
}

// WithFormat sets the default audio format.
func (d *DashScope) WithFormat(f string) *DashScope {
	d.format = f
	return d
}

// WithHTTPClient sets a custom HTTP client (used by tests to inject a mock).
func (d *DashScope) WithHTTPClient(c *http.Client) *DashScope {
	d.httpClient = c
	return d
}

// ModelName returns the configured TTS model identifier.
func (d *DashScope) ModelName() string {
	return d.model
}

type dashscopeTTSRequest struct {
	Model      string                 `json:"model"`
	Input      dashscopeTTSInput      `json:"input"`
	Parameters dashscopeTTSParameters `json:"parameters"`
}

type dashscopeTTSInput struct {
	Text string `json:"text"`
}

type dashscopeTTSParameters struct {
	Voice      string `json:"voice,omitempty"`
	Format     string `json:"format,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Speed      int    `json:"speed,omitempty"`
}

type dashscopeTTSResponse struct {
	Output struct {
		Audio     string `json:"audio"`
		RequestID string `json:"request_id"`
	} `json:"output"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Synthesize converts text to speech via the DashScope generation API. The
// returned Response carries the full decoded audio with IsLast=true.
func (d *DashScope) Synthesize(ctx context.Context, text string, opts Options) (*Response, error) {
	if d.apiKey == "" {
		return nil, fmt.Errorf("tts dashscope: missing api key")
	}
	merged := mergeOptions(Options{Voice: d.voice, Format: d.format}, opts)

	reqBody := dashscopeTTSRequest{
		Model: d.model,
		Input: dashscopeTTSInput{Text: text},
		Parameters: dashscopeTTSParameters{
			Voice:  merged.Voice,
			Format: merged.Format,
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tts dashscope: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.baseURL+"/api/v1/services/aigc/text2audio/generation", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("tts dashscope: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts dashscope: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts dashscope: %s: %s", resp.Status, string(body))
	}

	var parsed dashscopeTTSResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("tts dashscope: decode response: %w", err)
	}
	if parsed.Code != "" {
		return nil, fmt.Errorf("tts dashscope: %s: %s", parsed.Code, parsed.Message)
	}
	if parsed.Output.Audio == "" {
		return nil, fmt.Errorf("tts dashscope: empty audio in response (request_id=%s)", parsed.RequestID)
	}

	audio, err := base64.StdEncoding.DecodeString(parsed.Output.Audio)
	if err != nil {
		return nil, fmt.Errorf("tts dashscope: decode base64 audio: %w", err)
	}
	return &Response{
		Audio:     audio,
		MediaType: mediaTypeForFormat(merged.Format),
		IsLast:    true,
	}, nil
}
