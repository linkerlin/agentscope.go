package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func captureLogger(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})
	return slog.New(h), buf
}

func TestNew_JSONFormat(t *testing.T) {
	// New writes to os.Stdout; verify it doesn't panic and returns a logger.
	l := New(slog.LevelInfo, "json")
	l.Info("test", "key", "val")
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")
	l := NewFromEnv()
	if l == nil {
		t.Fatal("nil logger")
	}
}

func TestDefault_Singleton(t *testing.T) {
	SetDefault(nil)
	defaultLoggerOnce = false
	d1 := Default()
	d2 := Default()
	if d1 != d2 {
		t.Fatal("Default should return the same instance after first init")
	}
}

func TestSetDefault(t *testing.T) {
	custom := New(slog.LevelError, "json")
	SetDefault(custom)
	if Default() != custom {
		t.Fatal("SetDefault should override")
	}
}

func TestFromContext_Fallback(t *testing.T) {
	SetDefault(nil)
	defaultLoggerOnce = false
	if FromContext(context.Background()) == nil {
		t.Fatal("FromContext should fall back to Default, not nil")
	}
}

func TestFromContext_Stored(t *testing.T) {
	l, _ := captureLogger(slog.LevelInfo)
	ctx := WithLogger(context.Background(), l)
	if FromContext(ctx) != l {
		t.Fatal("FromContext should return the stored logger")
	}
}

func TestDiscard(t *testing.T) {
	d := Discard()
	d.Info("dropped") // must not panic or write anywhere
}

func TestLevelFromEnv(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo, // default
		"bogus": slog.LevelInfo, // bad → default
	}
	for env, want := range cases {
		t.Setenv("LOG_LEVEL", env)
		if got := levelFromEnv(); got != want {
			t.Errorf("LOG_LEVEL=%q: got %v want %v", env, got, want)
		}
	}
}

func TestFormatFromEnv(t *testing.T) {
	t.Setenv("LOG_FORMAT", "text")
	if formatFromEnv() != "text" {
		t.Fatal("text format not detected")
	}
	t.Setenv("LOG_FORMAT", "")
	if formatFromEnv() != "json" {
		t.Fatal("default format should be json")
	}
}

func TestKeyConstants(t *testing.T) {
	if KeyAgentID != "agent_id" || KeySessionID != "session_id" {
		t.Fatal("key constants wrong")
	}
}

func TestSlogLoggerSatisfiesInterface(t *testing.T) {
	l, buf := captureLogger(slog.LevelInfo)
	var ifce Logger = l
	ifce.Info("hello", "agent_id", "a1")
	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)
	if parsed["msg"] != "hello" {
		t.Fatalf("unexpected log: %s", buf.String())
	}
	if parsed["agent_id"] != "a1" {
		t.Fatalf("attr missing: %s", buf.String())
	}
}

func TestNew_TextFormat(t *testing.T) {
	// redirect stdout briefly
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()
	New(slog.LevelInfo, "text").Info("texttest")
	w.Close()
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "texttest") {
		t.Fatal("text format should write the message")
	}
}
