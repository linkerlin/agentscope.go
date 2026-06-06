package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY")
		return
	}

	// ---- 1. Chat model for reasoning ----
	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		panic(err)
	}

	// ---- 2. Audio model for TTS / STT ----
	audioModel := model.NewOpenAITTS(apiKey).WithVoice("alloy")

	// ---- 3. Voice-enabled ReActAgent ----
	agent, err := react.Builder().
		Name("VoiceAgent").
		SysPrompt("You are a helpful voice assistant. Keep replies concise and natural.").
		Model(chatModel).
		Build()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// ---- 4. Simulate: user audio -> text (STT) ----
	// In a real app this would come from a microphone.
	// Here we load a sample audio file or create a dummy one.
	userAudioPath := os.Getenv("USER_AUDIO_PATH")
	var userText string
	if userAudioPath != "" {
		audioData, err := os.ReadFile(userAudioPath)
		if err != nil {
			panic(err)
		}
		userText, err = audioModel.TranscribeSpeech(ctx, audioData, model.AudioOptions{Format: "mp3", Language: "en"})
		if err != nil {
			panic(err)
		}
		fmt.Printf("User (transcribed): %s\n", userText)
	} else {
		userText = "Hello, what's the weather like today?"
		fmt.Printf("User (text fallback): %s\n", userText)
	}

	// ---- 5. Agent reasoning ----
	msg := message.NewMsg().Role(message.RoleUser).TextContent(userText).Build()
	reply, err := agent.Call(ctx, msg)
	if err != nil {
		panic(err)
	}
	replyText := reply.GetTextContent()
	fmt.Printf("Agent: %s\n", replyText)

	// ---- 6. Text -> audio (TTS) ----
	audioOut, err := audioModel.SynthesizeSpeech(ctx, replyText, model.AudioOptions{Format: "mp3"})
	if err != nil {
		panic(err)
	}

	outPath := "reply.mp3"
	if err := os.WriteFile(outPath, audioOut, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Reply audio saved to %s (%d bytes)\n", outPath, len(audioOut))
	fmt.Println("\nIn a full Voice Agent deployment:")
	fmt.Println("  - Use pion/webrtc or a native audio recorder to capture microphone input")
	fmt.Println("  - Stream audio chunks to TranscribeSpeech for real-time STT")
	fmt.Println("  - Play back synthesized audio via oto/malgo or WebRTC data channels")
}
