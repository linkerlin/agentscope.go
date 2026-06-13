package onnx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WhisperAudioEmbedder Whisper 音频嵌入器（HTTP 代理方案）
type WhisperAudioEmbedder struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// WhisperAudioEmbedderConfig Whisper 嵌入器配置
type WhisperAudioEmbedderConfig struct {
	BaseURL string        `json:"base_url"`
	Timeout time.Duration `json:"timeout"`
	Model   string        `json:"model,omitempty"` // tiny, base, small, medium, large
}

// DefaultWhisperAudioEmbedderConfig 返回默认配置
func DefaultWhisperAudioEmbedderConfig() WhisperAudioEmbedderConfig {
	return WhisperAudioEmbedderConfig{
		BaseURL: "http://localhost:8000",
		Timeout: 60 * time.Second,
		Model:   "base",
	}
}

// NewWhisperAudioEmbedder 创建 Whisper 音频嵌入器
func NewWhisperAudioEmbedder(config WhisperAudioEmbedderConfig) *WhisperAudioEmbedder {
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	return &WhisperAudioEmbedder{
		BaseURL: config.BaseURL,
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
		Timeout: config.Timeout,
	}
}

// EmbedAudioRequest 音频嵌入请求
type EmbedAudioRequest struct {
	AudioData []float32 `json:"audio_data"` // Mel 频谱图 [1, 80, 3000]
	ModelName string    `json:"model_name,omitempty"`
	Task      string    `json:"task,omitempty"` // transcribe, translate
}

// EmbedAudioResponse 音频嵌入响应
type EmbedAudioResponse struct {
	Embedding []float32 `json:"embedding"`      // 音频嵌入向量
	Text      string    `json:"text,omitempty"` // 转录文本（如果服务支持）
	Model     string    `json:"model"`
	Dim       int       `json:"dim"`
}

// EmbedAudio 嵌入音频
// 输入: 预处理后的 Mel 频谱图（NCHW 格式）
// 输出: 音频嵌入向量
func (e *WhisperAudioEmbedder) EmbedAudio(audioData []float32) ([]float32, error) {
	req := EmbedAudioRequest{
		AudioData: audioData,
		ModelName: e.getModelName(),
		Task:      "transcribe",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("onnx: marshal audio embed request: %w", err)
	}

	url := e.BaseURL + "/embed/audio"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("onnx: create audio embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("onnx: audio embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("onnx: audio embed failed with status %d: %s", resp.StatusCode, string(body))
	}

	var embedResp EmbedAudioResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("onnx: decode audio embed response: %w", err)
	}

	return embedResp.Embedding, nil
}

// EmbedAudioFromPCM 从原始 PCM 数据嵌入（完整管道）
func (e *WhisperAudioEmbedder) EmbedAudioFromPCM(samples []float32, sampleRate int) ([]float32, error) {
	// 1. 预处理
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())
	preprocessed, err := preprocessor.Preprocess(samples, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("onnx: preprocess audio: %w", err)
	}

	// 2. 嵌入
	return e.EmbedAudio(preprocessed)
}

// TranscribeAudio 转录音频（返回文本）
func (e *WhisperAudioEmbedder) TranscribeAudio(audioData []float32) (string, error) {
	req := EmbedAudioRequest{
		AudioData: audioData,
		ModelName: e.getModelName(),
		Task:      "transcribe",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("onnx: marshal transcribe request: %w", err)
	}

	url := e.BaseURL + "/transcribe"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("onnx: create transcribe request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("onnx: transcribe request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("onnx: transcribe failed with status %d: %s", resp.StatusCode, string(body))
	}

	var embedResp EmbedAudioResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return "", fmt.Errorf("onnx: decode transcribe response: %w", err)
	}

	return embedResp.Text, nil
}

// BatchEmbedAudio 批量嵌入音频
func (e *WhisperAudioEmbedder) BatchEmbedAudio(audioDataList [][]float32) ([][]float32, error) {
	results := make([][]float32, len(audioDataList))
	for i, data := range audioDataList {
		embedding, err := e.EmbedAudio(data)
		if err != nil {
			return nil, fmt.Errorf("onnx: batch embed audio %d: %w", i, err)
		}
		results[i] = embedding
	}
	return results, nil
}

// HealthCheck 健康检查
func (e *WhisperAudioEmbedder) HealthCheck() error {
	url := e.BaseURL + "/health"
	resp, err := e.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("onnx: health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("onnx: health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetModelInfo 获取模型信息
func (e *WhisperAudioEmbedder) GetModelInfo() (map[string]any, error) {
	url := e.BaseURL + "/model/info"
	resp, err := e.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("onnx: get model info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("onnx: model info failed with status %d", resp.StatusCode)
	}

	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("onnx: decode model info: %w", err)
	}

	return info, nil
}

func (e *WhisperAudioEmbedder) getModelName() string {
	// 从配置中获取模型名称，这里简化处理
	return "whisper-base"
}

// AudioTextAlignment 音频-文本对齐（用于跨模态检索）
type AudioTextAlignment struct {
	AudioEmbedder *WhisperAudioEmbedder
	TextEmbedder  *CLIPTextEmbedder
}

// NewAudioTextAlignment 创建音频-文本对齐器
func NewAudioTextAlignment(audioEmbedder *WhisperAudioEmbedder, textEmbedder *CLIPTextEmbedder) *AudioTextAlignment {
	return &AudioTextAlignment{
		AudioEmbedder: audioEmbedder,
		TextEmbedder:  textEmbedder,
	}
}

// Align 对齐音频和文本
func (a *AudioTextAlignment) Align(audioData []float32, text string) (float32, error) {
	audioEmbedding, err := a.AudioEmbedder.EmbedAudio(audioData)
	if err != nil {
		return 0, fmt.Errorf("onnx: embed audio for alignment: %w", err)
	}

	textEmbedding, err := a.TextEmbedder.EmbedText(text)
	if err != nil {
		return 0, fmt.Errorf("onnx: embed text for alignment: %w", err)
	}

	return CrossModalSimilarity(audioEmbedding, textEmbedding)
}
