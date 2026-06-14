package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/openai"
)

// RealtimeVoiceAgent 实现端到端实时语音对话：
// 麦克风音频 → VAD → STT → LLM → TTS → 扬声器
// 支持打断（barge-in）：用户说话时自动停止 TTS 播放。
type RealtimeVoiceAgent struct {
	chatModel  model.ChatModel
	audioModel model.AudioModel
	agent      agent.Agent

	// 音频管道
	micIn  chan []byte // 麦克风原始音频输入
	vadOut chan []byte // VAD 检测到的语音片段
	sttOut chan string // STT 转录文本
	llmOut chan string // LLM 回复文本
	ttsOut chan []byte // TTS 合成音频

	// 控制信号
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 打断检测
	isSpeaking bool // 用户是否在说话
	mu         sync.RWMutex

	// 配置
	sampleRate   int
	vadThreshold float64
}

// NewRealtimeVoiceAgent 创建实时语音 Agent。
func NewRealtimeVoiceAgent(chatModel model.ChatModel, audioModel model.AudioModel) *RealtimeVoiceAgent {
	agent, _ := react.Builder().
		Name("RealtimeVoiceAgent").
		SysPrompt("You are a helpful voice assistant. Keep replies concise and natural. Respond quickly.").
		Model(chatModel).
		Build()

	ctx, cancel := context.WithCancel(context.Background())
	rva := &RealtimeVoiceAgent{
		agent:        agent,
		chatModel:    chatModel,
		audioModel:   audioModel,
		micIn:        make(chan []byte, 100),
		vadOut:       make(chan []byte, 10),
		sttOut:       make(chan string, 10),
		llmOut:       make(chan string, 10),
		ttsOut:       make(chan []byte, 10),
		ctx:          ctx,
		cancel:       cancel,
		sampleRate:   16000,
		vadThreshold: 0.02,
	}
	return rva
}

// Start 启动所有处理管道。
func (rva *RealtimeVoiceAgent) Start() {
	// 1. VAD 处理：从 micIn 读取，检测语音活动，输出到 vadOut
	rva.wg.Add(1)
	go rva.vadWorker()

	// 2. STT 处理：从 vadOut 读取语音片段，转录为文本
	rva.wg.Add(1)
	go rva.sttWorker()

	// 3. LLM 处理：从 sttOut 读取文本，调用 Agent 推理
	rva.wg.Add(1)
	go rva.llmWorker()

	// 4. TTS 处理：从 llmOut 读取文本，合成音频
	rva.wg.Add(1)
	go rva.ttsWorker()

	fmt.Println("[RealtimeVoiceAgent] All pipelines started")
	fmt.Println("  - VAD: energy-based voice activity detection")
	fmt.Println("  - STT: OpenAI Whisper streaming")
	fmt.Println("  - LLM: ReAct Agent reasoning")
	fmt.Println("  - TTS: OpenAI TTS streaming")
	fmt.Println("  - Barge-in: user speech interrupts assistant playback")
}

// Stop 停止所有管道。
func (rva *RealtimeVoiceAgent) Stop() {
	rva.cancel()
	close(rva.micIn)
	rva.wg.Wait()
	fmt.Println("[RealtimeVoiceAgent] All pipelines stopped")
}

// FeedAudio 向麦克风输入管道送入音频数据。
func (rva *RealtimeVoiceAgent) FeedAudio(data []byte) {
	select {
	case rva.micIn <- data:
	case <-rva.ctx.Done():
	}
}

// ReadTTS 从 TTS 输出管道读取合成音频。
func (rva *RealtimeVoiceAgent) ReadTTS() <-chan []byte {
	return rva.ttsOut
}

// IsUserSpeaking 返回用户是否正在说话（用于 UI 显示）。
func (rva *RealtimeVoiceAgent) IsUserSpeaking() bool {
	rva.mu.RLock()
	defer rva.mu.RUnlock()
	return rva.isSpeaking
}

// vadWorker 语音活动检测工作器。
func (rva *RealtimeVoiceAgent) vadWorker() {
	defer rva.wg.Done()

	var buffer []byte
	var inSpeech bool
	const frameSize = 320 // 20ms @ 16kHz, 16-bit

	for {
		select {
		case data, ok := <-rva.micIn:
			if !ok {
				if len(buffer) > 0 {
					rva.vadOut <- buffer
				}
				close(rva.vadOut)
				return
			}
			buffer = append(buffer, data...)

			// 处理完整帧
			for len(buffer) >= frameSize {
				frame := buffer[:frameSize]
				buffer = buffer[frameSize:]

				energy := rva.computeEnergy(frame)
				if energy > rva.vadThreshold {
					if !inSpeech {
						inSpeech = true
						rva.mu.Lock()
						rva.isSpeaking = true
						rva.mu.Unlock()
						fmt.Println("[VAD] Speech detected")
					}
				} else if inSpeech {
					// 语音结束，发送完整片段
					inSpeech = false
					rva.mu.Lock()
					rva.isSpeaking = false
					rva.mu.Unlock()
					// 这里简化处理：直接发送当前 buffer
				}
			}

		case <-rva.ctx.Done():
			close(rva.vadOut)
			return
		}
	}
}

// computeEnergy 计算音频帧的能量（简化版 VAD）。
func (rva *RealtimeVoiceAgent) computeEnergy(frame []byte) float64 {
	if len(frame) < 2 {
		return 0
	}
	// 16-bit PCM: 计算 RMS
	var sum float64
	for i := 0; i < len(frame)-1; i += 2 {
		sample := int16(frame[i]) | int16(frame[i+1])<<8
		sum += float64(sample) * float64(sample)
	}
	count := float64(len(frame) / 2)
	if count == 0 {
		return 0
	}
	return sum / count / 32768.0 / 32768.0
}

// sttWorker 语音转文字工作器。
func (rva *RealtimeVoiceAgent) sttWorker() {
	defer rva.wg.Done()

	var accum []byte
	lastSpeech := time.Now()
	const silenceTimeout = 500 * time.Millisecond

	for {
		select {
		case data, ok := <-rva.vadOut:
			if !ok {
				// 处理剩余累积音频
				if len(accum) > 0 {
					rva.transcribeAndSend(accum)
				}
				close(rva.sttOut)
				return
			}
			accum = append(accum, data...)
			lastSpeech = time.Now()

		case <-time.After(silenceTimeout):
			if len(accum) > 0 && time.Since(lastSpeech) > silenceTimeout {
				rva.transcribeAndSend(accum)
				accum = nil
			}

		case <-rva.ctx.Done():
			close(rva.sttOut)
			return
		}
	}
}

// transcribeAndSend 转录音频并发送到 LLM 管道。
func (rva *RealtimeVoiceAgent) transcribeAndSend(audio []byte) {
	if len(audio) < 1600 { // 至少 100ms
		return
	}
	text, err := rva.audioModel.TranscribeSpeech(rva.ctx, audio, model.AudioOptions{
		Format:   "pcm",
		Language: "zh",
	})
	if err != nil {
		fmt.Printf("[STT] Error: %v\n", err)
		return
	}
	if text == "" {
		return
	}
	fmt.Printf("[STT] Transcribed: %s\n", text)

	// 打断检测：如果用户在说话，清空 TTS 输出（barge-in）
	rva.mu.RLock()
	isSpeaking := rva.isSpeaking
	rva.mu.RUnlock()
	if isSpeaking {
		fmt.Println("[Barge-in] User interrupted, clearing TTS queue")
		// 清空 TTS 输出通道
		select {
		case <-rva.ttsOut:
		default:
		}
	}

	select {
	case rva.sttOut <- text:
	case <-rva.ctx.Done():
	}
}

// llmWorker LLM 推理工作器。
func (rva *RealtimeVoiceAgent) llmWorker() {
	defer rva.wg.Done()

	for {
		select {
		case text, ok := <-rva.sttOut:
			if !ok {
				close(rva.llmOut)
				return
			}
			msg := message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
			reply, err := rva.agent.Call(rva.ctx, msg)
			if err != nil {
				fmt.Printf("[LLM] Error: %v\n", err)
				continue
			}
			replyText := reply.GetTextContent()
			fmt.Printf("[LLM] Reply: %s\n", replyText)

			select {
			case rva.llmOut <- replyText:
			case <-rva.ctx.Done():
				return
			}

		case <-rva.ctx.Done():
			close(rva.llmOut)
			return
		}
	}
}

// ttsWorker 文字转语音工作器。
func (rva *RealtimeVoiceAgent) ttsWorker() {
	defer rva.wg.Done()

	for {
		select {
		case text, ok := <-rva.llmOut:
			if !ok {
				close(rva.ttsOut)
				return
			}
			// 检查打断：如果用户正在说话，跳过此回复
			rva.mu.RLock()
			isSpeaking := rva.isSpeaking
			rva.mu.RUnlock()
			if isSpeaking {
				fmt.Println("[TTS] Skipped: user is speaking (barge-in)")
				continue
			}

			audio, err := rva.audioModel.SynthesizeSpeech(rva.ctx, text, model.AudioOptions{
				Format: "mp3",
				Voice:  "alloy",
			})
			if err != nil {
				fmt.Printf("[TTS] Error: %v\n", err)
				continue
			}
			fmt.Printf("[TTS] Synthesized: %d bytes\n", len(audio))

			select {
			case rva.ttsOut <- audio:
			case <-rva.ctx.Done():
				return
			}

		case <-rva.ctx.Done():
			close(rva.ttsOut)
			return
		}
	}
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY")
		return
	}

	// 1. Chat model
	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		panic(err)
	}

	// 2. Audio model (TTS + STT)
	audioModel := model.NewOpenAITTS(apiKey).WithVoice("alloy")

	// 3. 创建实时语音 Agent
	rva := NewRealtimeVoiceAgent(chatModel, audioModel)
	rva.Start()
	defer rva.Stop()

	// 4. 模拟音频输入（实际应用中从麦克风读取）
	fmt.Println("\n=== Simulating voice conversation ===")
	fmt.Println("(In production: use pion/webrtc or portaudio for real microphone input)")
	fmt.Println()

	// 模拟用户说 "Hello"
	simulateUserSpeech(rva, "Hello, what can you help me with?")

	// 等待回复
	time.Sleep(2 * time.Second)

	// 模拟打断：用户说 "Wait, actually..."
	fmt.Println("\n--- User interrupts (barge-in) ---")
	simulateUserSpeech(rva, "Wait, actually tell me a joke instead.")

	// 等待回复
	time.Sleep(3 * time.Second)

	fmt.Println("\n=== Conversation ended ===")
}

// simulateUserSpeech 模拟用户语音输入（用于测试）。
func simulateUserSpeech(rva *RealtimeVoiceAgent, text string) {
	fmt.Printf("[User] %s\n", text)
	// 模拟音频数据：生成一些非零 PCM 数据
	fakeAudio := make([]byte, 3200) // 100ms @ 16kHz
	for i := range fakeAudio {
		fakeAudio[i] = byte(i % 256)
	}
	rva.FeedAudio(fakeAudio)

	// 直接通过 STT 管道发送文本（测试模式绕过 VAD）
	go func() {
		select {
		case rva.sttOut <- text:
		case <-rva.ctx.Done():
		}
	}()
}
