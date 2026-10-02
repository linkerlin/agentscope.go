// tts/gemini.go — a non-realtime tts.Model backed by Google Gemini TTS
// (gemini-2.5-*-tts): generateContent with AUDIO response modality returns
// inline PCM audio. Mirrors the DashScope backend's shape (19.8).
package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Gemini implements tts.Model via the Generative Language generateContent
// API with responseModalities=["AUDIO"].
type Gemini struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string // gemini-2.5-flash-preview-tts | gemini-2.5-pro-tts | ...
	voice      string // prebuilt voice name, e.g. "Kore"
}

// NewGemini creates a Gemini TTS model with sensible defaults.
func NewGemini(apiKey string) *Gemini {
	return &Gemini{
		apiKey:     apiKey,
		baseURL:    "https://generativelanguage.googleapis.com",
		httpClient: &http.Client{Timeout: 60 * time.Second},
		model:      "gemini-2.5-flash-preview-tts",
		voice:      "Kore",
	}
}

// WithBaseURL overrides the API base URL (e.g. for a proxy or test server).
func (g *Gemini) WithBaseURL(url string) *Gemini {
	g.baseURL = url
	return g
}

// WithModel sets the TTS model name.
func (g *Gemini) WithModel(m string) *Gemini {
	g.model = m
	return g
}

// WithVoice sets the default prebuilt voice.
func (g *Gemini) WithVoice(v string) *Gemini {
	g.voice = v
	return g
}

// WithHTTPClient sets a custom HTTP client (used by tests to inject a mock).
func (g *Gemini) WithHTTPClient(c *http.Client) *Gemini {
	g.httpClient = c
	return g
}

// ModelName returns the configured TTS model identifier.
func (g *Gemini) ModelName() string { return g.model }

type geminiTTSRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	ResponseModalities []string         `json:"responseModalities"`
	SpeechConfig       *geminiSpeechCfg `json:"speechConfig,omitempty"`
}

type geminiSpeechCfg struct {
	VoiceConfig geminiVoiceCfg `json:"voiceConfig"`
}

type geminiVoiceCfg struct {
	PrebuiltVoiceConfig struct {
		VoiceName string `json:"voiceName"`
	} `json:"prebuiltVoiceConfig"`
}

type geminiTTSResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// Synthesize converts text to speech via generateContent. Gemini TTS returns
// inline 24kHz PCM (audio/pcm;rate=24000) regardless of the requested format
// token; the response MediaType reflects that wire reality.
func (g *Gemini) Synthesize(ctx context.Context, text string, opts Options) (*Response, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("tts gemini: missing api key")
	}
	merged := mergeOptions(Options{Voice: g.voice}, opts)

	reqBody := geminiTTSRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: text}}}},
		GenerationConfig: geminiGenerationConfig{
			ResponseModalities: []string{"AUDIO"},
		},
	}
	if merged.Voice != "" {
		reqBody.GenerationConfig.SpeechConfig = &geminiSpeechCfg{}
		reqBody.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName = merged.Voice
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tts gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", g.baseURL, g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("tts gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts gemini: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts gemini: %s: %s", resp.Status, geminiErrorMessage(body))
	}
	var parsed geminiTTSResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("tts gemini: decode response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("tts gemini: no audio candidate in response")
	}
	part := parsed.Candidates[0].Content.Parts[0].InlineData
	if part.Data == "" {
		return nil, fmt.Errorf("tts gemini: empty audio in response")
	}
	audio, err := base64.StdEncoding.DecodeString(part.Data)
	if err != nil {
		return nil, fmt.Errorf("tts gemini: decode base64 audio: %w", err)
	}
	mediaType := part.MimeType
	if i := strings.Index(mediaType, ";"); i >= 0 {
		mediaType = mediaType[:i] // strip ";rate=24000" for the MIME type
	}
	return &Response{Audio: audio, MediaType: mediaType, IsLast: true}, nil
}

// geminiErrorMessage extracts {"error":{"message"}} when present, so error
// surfaces carry the server's reason instead of a bare status.
func geminiErrorMessage(body []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		if parsed.Error.Status != "" {
			return parsed.Error.Status + ": " + parsed.Error.Message
		}
		return parsed.Error.Message
	}
	return string(body)
}
