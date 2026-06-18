package middleware_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/tts"
)

// mockTTS is a controllable tts.Model for middleware tests.
type mockTTS struct {
	audio    []byte
	media    string
	callErr  error
	called   bool
	lastText string
}

func (m *mockTTS) ModelName() string { return "mock-tts" }

func (m *mockTTS) Synthesize(ctx context.Context, text string, opts tts.Options) (*tts.Response, error) {
	m.called = true
	m.lastText = text
	if m.callErr != nil {
		return nil, m.callErr
	}
	return &tts.Response{Audio: m.audio, MediaType: m.media, IsLast: true}, nil
}

func newAssistantMsg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(text).Build()
}

func TestTTSMiddleware_AttachesAudio(t *testing.T) {
	mock := &mockTTS{audio: []byte{1, 2, 3}, media: "audio/wav"}
	mw := middleware.NewTTSMiddleware(mock)
	chain := middleware.Classify([]middleware.Middleware{mw})

	handler := middleware.ChainReply(chain, stubAgent{name: "a"}, &middleware.ReplyInput{
		Messages: []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()},
	}, func(ctx context.Context) (*message.Msg, error) {
		return newAssistantMsg("hello there"), nil
	})

	msg, err := handler(context.Background())
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if !mock.called || mock.lastText != "hello there" {
		t.Fatalf("tts not called with reply text: called=%v text=%q", mock.called, mock.lastText)
	}
	// Expect an appended AudioBlock with base64-encoded audio.
	var foundAudio bool
	for _, b := range msg.Content {
		if ab, ok := b.(*message.AudioBlock); ok {
			foundAudio = true
			dec, _ := base64.StdEncoding.DecodeString(ab.Base64)
			if string(dec) != string(mock.audio) {
				t.Fatalf("decoded audio mismatch: %v", dec)
			}
			if ab.MimeType != "audio/wav" {
				t.Fatalf("unexpected mime: %s", ab.MimeType)
			}
		}
	}
	if !foundAudio {
		t.Fatal("expected an AudioBlock appended to the reply")
	}
}

func TestTTSMiddleware_StrictError(t *testing.T) {
	mock := &mockTTS{callErr: errors.New("synth failed")}
	mw := middleware.NewTTSMiddleware(mock).WithStrict()
	chain := middleware.Classify([]middleware.Middleware{mw})

	handler := middleware.ChainReply(chain, stubAgent{name: "a"}, &middleware.ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return newAssistantMsg("hi"), nil
	})
	if _, err := handler(context.Background()); err == nil {
		t.Fatal("expected strict middleware to propagate synthesis error")
	}
}

func TestTTSMiddleware_NonFatalErrorRecorded(t *testing.T) {
	mock := &mockTTS{callErr: errors.New("synth failed")}
	mw := middleware.NewTTSMiddleware(mock) // non-strict
	chain := middleware.Classify([]middleware.Middleware{mw})

	handler := middleware.ChainReply(chain, stubAgent{name: "a"}, &middleware.ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return newAssistantMsg("hi"), nil
	})
	msg, err := handler(context.Background())
	if err != nil {
		t.Fatalf("non-strict should not fail reply: %v", err)
	}
	if msg.Metadata["tts_error"] == nil {
		t.Fatal("expected tts_error recorded in metadata")
	}
}

func TestTTSMiddleware_SkipsNonAssistantAndEmpty(t *testing.T) {
	mock := &mockTTS{audio: []byte{9}}
	mw := middleware.NewTTSMiddleware(mock)
	chain := middleware.Classify([]middleware.Middleware{mw})

	// Non-assistant role -> skipped.
	h1 := middleware.ChainReply(chain, stubAgent{name: "a"}, &middleware.ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), nil
	})
	msg, _ := h1(context.Background())
	if mock.called {
		t.Fatal("tts should not be called for non-assistant messages")
	}
	for _, b := range msg.Content {
		if _, ok := b.(*message.AudioBlock); ok {
			t.Fatal("no audio block expected for non-assistant")
		}
	}

	// Empty assistant text -> skipped.
	mock.called = false
	h2 := middleware.ChainReply(chain, stubAgent{name: "a"}, &middleware.ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return newAssistantMsg(""), nil
	})
	_, _ = h2(context.Background())
	if mock.called {
		t.Fatal("tts should not be called for empty text")
	}
}
