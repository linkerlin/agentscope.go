// examples/console_voice — the 19.7 MANUAL SMOKE for real device I/O: live
// microphone capture and speaker playback over a DashScope qwen-omni
// realtime session, driven through console.VoiceSession.
//
// The automated acceptance (console/voice_test.go) runs on a fake transport
// (MockModel) and fake devices; this binary is the explicit manual smoke the
// acceptance defers real capture/playback to.
//
// This program does NO process spawning and owns no audio hardware: capture
// audio arrives on stdin (16kHz mono s16le PCM) and assistant audio is
// written to stdout (24kHz mono s16le PCM) — YOU wire the real devices with
// shell pipelines (sox shown below; any equivalent works).
//
// Requirements: DASHSCOPE_API_KEY in the environment; sox on PATH.
//
// Usage (one line, three pipeline stages):
//
//	rec -q -t raw -r 16000 -b 16 -e signed-integer -c 1 - | \
//	  go run ./examples/console_voice --say "你好" | \
//	  play -q -t raw -r 24000 -b 16 -e signed-integer -c 1 -
//
// --say sends one text turn first (optional). Ctrl+C stops the pipeline;
// speaking over the assistant barge-ins automatically (server VAD reports
// the interrupt).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/linkerlin/agentscope.go/console"
	"github.com/linkerlin/agentscope.go/realtime"
	"github.com/linkerlin/agentscope.go/realtime/dashscope"
)

var pcm16k = realtime.AudioFormat{Codec: "pcm", SampleRate: 16000, Channels: 1}

// stdinMic forwards stdin PCM as microphone input — the shell pipeline's
// capture stage (sox rec) is the actual device. This is the manual-smoke
// device: no automated test drives it.
type stdinMic struct {
	stopped chan struct{}
}

func (m *stdinMic) Start(ctx context.Context, emit func(pcm []byte)) error {
	go func() {
		buf := make([]byte, 320*2) // 20ms frames at 16kHz s16le mono
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				emit(buf[:n])
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "[mic] read: %v\n", err)
				}
				return
			}
			select {
			case <-m.stopped:
				return
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	return nil
}

func (m *stdinMic) Stop() error {
	select {
	case <-m.stopped:
	default:
		close(m.stopped)
	}
	return nil
}

// stdoutSink writes assistant audio to stdout — the shell pipeline's
// playback stage (sox play) is the actual speaker. Playback consumption is
// approximated wall-clock so the shared playout clock keeps moving and a
// barge-in cuts at the heard position.
type stdoutSink struct {
	session *console.VoiceSession
}

func (s *stdoutSink) Play(seq int, chunk []byte) {
	if _, err := os.Stdout.Write(chunk); err != nil {
		return
	}
	n := len(chunk)
	go func() {
		// 24kHz s16le mono = 48000 bytes/s: report the chunk as consumed
		// after its wall-clock playback duration.
		time.Sleep(time.Duration(n) * time.Second / 48000)
		if s.session != nil {
			s.session.Playout().Advance(n)
		}
	}()
}

func main() {
	say := flag.String("say", "", "optional text turn sent before entering voice mode")
	flag.Parse()

	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "DASHSCOPE_API_KEY not set (this is the MANUAL device smoke)")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sink := &stdoutSink{}
	sess, err := console.StartVoiceSession(ctx, console.VoiceOptions{
		Model: dashscope.NewQwenOmniRealtime(apiKey),
		Offer: realtime.NegotiateOffer{Formats: []realtime.AudioFormat{pcm16k}},
		Mic:   &stdinMic{stopped: make(chan struct{})},
		Sink:  sink,
		VAD:   realtime.NewVAD(realtime.DefaultVADConfig()),
		OnText: func(delta string) {
			fmt.Fprintf(os.Stderr, "%s", delta) // captions: stderr keeps stdout pure audio
		},
		OnUser: func(text string) {
			fmt.Fprintf(os.Stderr, "\n[你] %s\n", text)
		},
		OnError: func(msg string) {
			fmt.Fprintf(os.Stderr, "\n[错误] %s\n", msg)
		},
		OnClosed: func() {
			fmt.Fprintln(os.Stderr, "\n[会话结束]")
			cancel()
		},
		// No terminal in a piped smoke: tools pass through, each printed.
		ConfirmTool: func(call realtime.ToolCall) (bool, []byte) {
			fmt.Fprintf(os.Stderr, "\n[工具] %s(%s)\n", call.Name, call.Args)
			return true, []byte(`{"ok":true}`)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	sink.session = sess
	defer sess.Close()

	if *say != "" {
		if err := sess.SubmitText(*say); err != nil {
			fmt.Fprintf(os.Stderr, "say: %v\n", err)
		}
	}
	if err := sess.SetMode(console.VoiceModeVoice); err != nil {
		fmt.Fprintf(os.Stderr, "voice mode: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "[语音模式] 对着麦克风说话即可；Ctrl+C 退出；打断靠直接开口")

	<-ctx.Done()
}
