package model

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestMockAudioModel_InterfaceCompliance(t *testing.T) {
	var _ AudioModel = (*MockAudioModel)(nil)
}

func TestMockAudioModel_Chat(t *testing.T) {
	m := NewMockAudioModel("mock-audio")
	resp, err := m.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestMockAudioModel_ChatStream(t *testing.T) {
	m := NewMockAudioModel("mock-audio")
	ch, err := m.ChatStream(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var chunks []string
	for c := range ch {
		if c.Delta != "" {
			chunks = append(chunks, c.Delta)
		}
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestMockAudioModel_SynthesizeSpeech(t *testing.T) {
	m := NewMockAudioModel("mock-audio")
	data, err := m.SynthesizeSpeech(context.Background(), "hello", AudioOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "mock-audio-data" {
		t.Fatalf("unexpected audio: %s", string(data))
	}
}

func TestMockAudioModel_TranscribeSpeech(t *testing.T) {
	m := NewMockAudioModel("mock-audio")
	text, err := m.TranscribeSpeech(context.Background(), []byte("audio"), AudioOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if text != "mock transcription" {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestMockAudioModel_StreamAudio(t *testing.T) {
	m := NewMockAudioModel("mock-audio")
	audioIn := make(chan []byte, 1)
	textOut := make(chan string, 1)

	audioIn <- []byte("chunk1")
	close(audioIn)

	ctx := context.Background()
	go func() {
		if err := m.StreamAudio(ctx, audioIn, textOut); err != nil {
			t.Logf("stream audio error: %v", err)
		}
		close(textOut)
	}()

	var texts []string
	for text := range textOut {
		texts = append(texts, text)
	}
	if len(texts) != 1 || texts[0] != "Hello, I heard you." {
		t.Fatalf("unexpected texts: %v", texts)
	}
}

func TestMockAudioModel_StreamAudio_NoAudio(t *testing.T) {
	m := NewMockAudioModel("mock-audio")
	audioIn := make(chan []byte)
	close(audioIn)

	textOut := make(chan string, 1)
	ctx := context.Background()
	if err := m.StreamAudio(ctx, audioIn, textOut); err == nil {
		t.Fatal("expected error for no audio")
	}
}

func TestMockAudioModel_FluentAPI(t *testing.T) {
	m := NewMockAudioModel("test").
		WithChatResp(message.NewMsg().Role(message.RoleAssistant).TextContent("custom").Build()).
		WithSynthesizeResp([]byte("custom-audio")).
		WithTranscribeResp("custom-text")

	resp, _ := m.Chat(context.Background(), nil)
	if resp.GetTextContent() != "custom" {
		t.Fatal("custom chat resp not applied")
	}
	data, _ := m.SynthesizeSpeech(context.Background(), "", AudioOptions{})
	if string(data) != "custom-audio" {
		t.Fatal("custom synthesize resp not applied")
	}
	text, _ := m.TranscribeSpeech(context.Background(), nil, AudioOptions{})
	if text != "custom-text" {
		t.Fatal("custom transcribe resp not applied")
	}
}
