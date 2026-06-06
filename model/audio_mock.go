package model

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/message"
)

// MockAudioModel is a test double for AudioModel.
type MockAudioModel struct {
	name           string
	chatResp       *message.Msg
	synthesizeResp []byte
	transcribeResp string
	streamAudioFn  func(ctx context.Context, audioIn <-chan []byte, textOut chan<- string) error
}

// NewMockAudioModel creates a new MockAudioModel.
func NewMockAudioModel(name string) *MockAudioModel {
	return &MockAudioModel{
		name:           name,
		chatResp:       message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
		synthesizeResp: []byte("mock-audio-data"),
		transcribeResp: "mock transcription",
	}
}

func (m *MockAudioModel) WithChatResp(resp *message.Msg) *MockAudioModel {
	m.chatResp = resp
	return m
}

func (m *MockAudioModel) WithSynthesizeResp(data []byte) *MockAudioModel {
	m.synthesizeResp = data
	return m
}

func (m *MockAudioModel) WithTranscribeResp(text string) *MockAudioModel {
	m.transcribeResp = text
	return m
}

func (m *MockAudioModel) WithStreamAudio(fn func(ctx context.Context, audioIn <-chan []byte, textOut chan<- string) error) *MockAudioModel {
	m.streamAudioFn = fn
	return m
}

func (m *MockAudioModel) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	return m.chatResp, nil
}

func (m *MockAudioModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	ch := make(chan *StreamChunk, 2)
	ch <- &StreamChunk{Delta: m.chatResp.GetTextContent(), Done: true}
	close(ch)
	return ch, nil
}

func (m *MockAudioModel) ModelName() string { return m.name }

func (m *MockAudioModel) SynthesizeSpeech(ctx context.Context, text string, opts AudioOptions) ([]byte, error) {
	return m.synthesizeResp, nil
}

func (m *MockAudioModel) TranscribeSpeech(ctx context.Context, audio []byte, opts AudioOptions) (string, error) {
	return m.transcribeResp, nil
}

func (m *MockAudioModel) StreamAudio(ctx context.Context, audioIn <-chan []byte, textOut chan<- string) error {
	if m.streamAudioFn != nil {
		return m.streamAudioFn(ctx, audioIn, textOut)
	}
	// Default: echo back a greeting after receiving first chunk.
	for range audioIn {
		textOut <- "Hello, I heard you."
		return nil
	}
	return fmt.Errorf("no audio received")
}

// Compile-time check.
var _ AudioModel = (*MockAudioModel)(nil)
