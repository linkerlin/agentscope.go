package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAITTS_SynthesizeSpeech(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/speech" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Model  string  `json:"model"`
			Input  string  `json:"input"`
			Voice  string  `json:"voice"`
			Format string  `json:"response_format"`
			Speed  float64 `json:"speed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Input == "" {
			http.Error(w, `{"error":"input required"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-audio-data"))
	}))
	defer server.Close()

	m := NewOpenAITTS("test-key").WithBaseURL(server.URL)
	data, err := m.SynthesizeSpeech(context.Background(), "Hello world", AudioOptions{Voice: "echo", Format: "mp3", Speed: 1.2})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if string(data) != "fake-audio-data" {
		t.Fatalf("unexpected audio data: %s", string(data))
	}
}

func TestOpenAITTS_SynthesizeSpeechError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid voice"}`))
	}))
	defer server.Close()

	m := NewOpenAITTS("test-key").WithBaseURL(server.URL)
	_, err := m.SynthesizeSpeech(context.Background(), "Hello", AudioOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid voice") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAITTS_TranscribeSpeech(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Parse multipart form.
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		model := r.FormValue("model")
		if model != "whisper-1" {
			http.Error(w, `{"error":"wrong model"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"text": "Hello world"})
	}))
	defer server.Close()

	m := NewOpenAITTS("test-key").WithBaseURL(server.URL)
	text, err := m.TranscribeSpeech(context.Background(), []byte("fake-audio"), AudioOptions{Format: "mp3", Language: "en"})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if text != "Hello world" {
		t.Fatalf("unexpected transcription: %s", text)
	}
}

func TestOpenAITTS_ChatNotSupported(t *testing.T) {
	m := NewOpenAITTS("test-key")
	_, err := m.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "does not support chat") {
		t.Fatalf("expected not-supported error, got %v", err)
	}
	_, err = m.ChatStream(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "does not support chat streaming") {
		t.Fatalf("expected not-supported error, got %v", err)
	}
}

func TestOpenAITTS_StreamAudioNotSupported(t *testing.T) {
	m := NewOpenAITTS("test-key")
	err := m.StreamAudio(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "does not support real-time audio streaming") {
		t.Fatalf("expected not-supported error, got %v", err)
	}
}

func TestOpenAITTS_ModelName(t *testing.T) {
	m := NewOpenAITTS("test-key")
	if m.ModelName() != "tts-1" {
		t.Fatalf("expected model tts-1, got %s", m.ModelName())
	}
	m.WithModel("tts-1-hd")
	if m.ModelName() != "tts-1-hd" {
		t.Fatalf("expected model tts-1-hd, got %s", m.ModelName())
	}
}

func TestOpenAITTS_WithHTTPClient(t *testing.T) {
	m := NewOpenAITTS("test-key").WithHTTPClient(&http.Client{})
	if m.httpClient == nil {
		t.Fatal("expected http client to be set")
	}
}
