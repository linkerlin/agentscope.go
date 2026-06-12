package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/linkerlin/agentscope.go/model"
)

// WithAudioModel registers an AudioModel for /v1/audio/* endpoints.
func (s *Server) WithAudioModel(m model.AudioModel) *Server {
	s.audioModel = m
	return s
}

// RegisterAudioRoutes adds OpenAI-compatible TTS and STT endpoints.
func (s *Server) RegisterAudioRoutes() {
	if s.audioModel == nil {
		return
	}
	s.mux.HandleFunc("POST /v1/audio/speech", s.requireAuth(s.handleSpeech))
	s.mux.HandleFunc("POST /v1/audio/transcriptions", s.requireAuth(s.handleTranscription))
}

type speechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

func (s *Server) handleSpeech(w http.ResponseWriter, r *http.Request) {
	if s.audioModel == nil {
		http.Error(w, "audio model not configured", http.StatusServiceUnavailable)
		return
	}

	var req speechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Input == "" {
		http.Error(w, "input is required", http.StatusBadRequest)
		return
	}

	format := req.ResponseFormat
	if format == "" {
		format = "mp3"
	}

	audio, err := s.audioModel.SynthesizeSpeech(r.Context(), req.Input, model.AudioOptions{
		Voice:  req.Voice,
		Format: format,
		Speed:  req.Speed,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("speech: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", audioContentType(format))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audio)))
	_, _ = w.Write(audio)
}

func audioContentType(format string) string {
	switch format {
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}

type transcriptionRequest struct {
	Model    string `json:"model,omitempty"`
	Language string `json:"language,omitempty"`
}

func (s *Server) handleTranscription(w http.ResponseWriter, r *http.Request) {
	if s.audioModel == nil {
		http.Error(w, "audio model not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("file: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	audio, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("read audio: %v", err), http.StatusInternalServerError)
		return
	}

	lang := r.FormValue("language")
	format := r.FormValue("response_format")
	if format == "" {
		format = "mp3"
	}

	text, err := s.audioModel.TranscribeSpeech(r.Context(), audio, model.AudioOptions{
		Language: lang,
		Format:   format,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("transcribe: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"text": text})
}
