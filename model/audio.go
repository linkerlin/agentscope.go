package model

import "context"

// AudioOptions controls audio synthesis / streaming parameters.
type AudioOptions struct {
	Voice   string
	Format  string // mp3 | wav | pcm | opus
	Speed   float64
	Language string
}

// AudioModel is a model that supports voice input/output.
// This is a V3 forward-looking interface; most ChatModel implementations
// will not satisfy it until a voice backend is plugged in.
type AudioModel interface {
	ChatModel

	// SynthesizeSpeech converts text to audio bytes.
	SynthesizeSpeech(ctx context.Context, text string, opts AudioOptions) ([]byte, error)

	// TranscribeSpeech converts audio bytes to text.
	TranscribeSpeech(ctx context.Context, audio []byte, opts AudioOptions) (string, error)

	// StreamAudio initiates a real-time bidirectional audio stream.
	// audioIn carries microphone chunks from the user.
	// textOut carries transcribed user utterances.
	// The caller is responsible for closing audioIn when done.
	StreamAudio(ctx context.Context, audioIn <-chan []byte, textOut chan<- string) error
}
