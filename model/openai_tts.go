package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// OpenAITTS implements AudioModel using OpenAI's TTS and Whisper APIs.
type OpenAITTS struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string // tts-1 | tts-1-hd
	voice      string
}

// NewOpenAITTS creates an OpenAI TTS adapter.
func NewOpenAITTS(apiKey string) *OpenAITTS {
	return &OpenAITTS{
		apiKey:     apiKey,
		baseURL:    "https://api.openai.com/v1",
		httpClient: &http.Client{Timeout: 60 * time.Second},
		model:      "tts-1",
		voice:      "alloy",
	}
}

// WithBaseURL sets a custom base URL (e.g. for proxies).
func (m *OpenAITTS) WithBaseURL(url string) *OpenAITTS {
	m.baseURL = url
	return m
}

// WithModel sets the TTS model.
func (m *OpenAITTS) WithModel(model string) *OpenAITTS {
	m.model = model
	return m
}

// WithVoice sets the default voice.
func (m *OpenAITTS) WithVoice(voice string) *OpenAITTS {
	m.voice = voice
	return m
}

// WithHTTPClient sets a custom HTTP client.
func (m *OpenAITTS) WithHTTPClient(c *http.Client) *OpenAITTS {
	m.httpClient = c
	return m
}

// --- ChatModel (not supported) ---

func (m *OpenAITTS) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	return nil, errors.New("OpenAITTS does not support chat")
}

func (m *OpenAITTS) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	return nil, errors.New("OpenAITTS does not support chat streaming")
}

func (m *OpenAITTS) ModelName() string {
	return m.model
}

// --- AudioModel ---

// SynthesizeSpeech converts text to audio using OpenAI TTS.
func (m *OpenAITTS) SynthesizeSpeech(ctx context.Context, text string, opts AudioOptions) ([]byte, error) {
	voice := opts.Voice
	if voice == "" {
		voice = m.voice
	}
	format := opts.Format
	if format == "" {
		format = "mp3"
	}

	body := map[string]any{
		"model":           m.model,
		"input":           text,
		"voice":           voice,
		"response_format": format,
	}
	if opts.Speed > 0 {
		body["speed"] = opts.Speed
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai_tts: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/audio/speech", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("openai_tts: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai_tts: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai_tts: %s: %s", resp.Status, string(b))
	}

	return io.ReadAll(resp.Body)
}

// TranscribeSpeech converts audio to text using OpenAI Whisper.
func (m *OpenAITTS) TranscribeSpeech(ctx context.Context, audio []byte, opts AudioOptions) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "audio."+opts.Format)
	if err != nil {
		return "", fmt.Errorf("openai_tts: create form file: %w", err)
	}
	if _, err := fw.Write(audio); err != nil {
		return "", fmt.Errorf("openai_tts: write audio: %w", err)
	}
	_ = w.WriteField("model", "whisper-1")
	if opts.Language != "" {
		_ = w.WriteField("language", opts.Language)
	}
	_ = w.WriteField("response_format", "json")
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("openai_tts: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai_tts: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai_tts: %s: %s", resp.Status, string(b))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai_tts: decode response: %w", err)
	}
	return result.Text, nil
}

// StreamAudio is not supported by the standard OpenAI HTTP API.
func (m *OpenAITTS) StreamAudio(ctx context.Context, audioIn <-chan []byte, textOut chan<- string) error {
	return errors.New("OpenAITTS does not support real-time audio streaming")
}
