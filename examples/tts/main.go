// Example: TTS middleware (mirrors Python agentscope TTSMiddleware #1832).
//
// Demonstrates the tts.Model abstraction and TTSMiddleware attaching synthesized
// audio to the assistant reply, using an in-process mock TTS model (no network).
//
// Run: go run ./examples/tts
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tts"
)

type mockChat struct{}

func (m *mockChat) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("Hello! How can I help you today?").Build(), nil
}
func (m *mockChat) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, fmt.Errorf("not used")
}
func (m *mockChat) ModelName() string { return "mock-chat" }

// mockTTS synthesizes deterministic fake PCM bytes for any input.
type mockTTS struct{}

func (m *mockTTS) ModelName() string { return "mock-tts" }
func (m *mockTTS) Synthesize(ctx context.Context, text string, opts tts.Options) (*tts.Response, error) {
	fake := []byte{0x52, 0x49, 0x46, 0x46} // pretend WAV header
	return &tts.Response{Audio: fake, MediaType: "audio/wav", IsLast: true}, nil
}

func main() {
	ttsMW := middleware.NewTTSMiddleware(&mockTTS{}).WithOptions(tts.Options{Voice: "Cherry", Format: "wav"})

	a, err := react.Builder().
		Name("VoiceAgent").
		SysPrompt("You are a voice assistant.").
		Model(&mockChat{}).
		Middlewares(ttsMW).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("text:", resp.GetTextContent())

	for _, b := range resp.Content {
		if ab, ok := b.(*message.AudioBlock); ok {
			dec, _ := base64.StdEncoding.DecodeString(ab.Base64)
			fmt.Printf("audio block: mime=%s bytes=%d decoded=%v\n", ab.MimeType, len(dec), dec)
		}
	}

	// List available TTS model cards (embedded YAML).
	fmt.Println("\nTTS model cards:")
	cards, _ := tts.ListModelCards()
	for _, c := range cards {
		fmt.Printf("  - %s (%s, realtime=%v)\n", c.ID, c.Provider, c.Realtime)
	}
}
