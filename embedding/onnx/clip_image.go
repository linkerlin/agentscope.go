package onnx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CLIPImageEmbedder CLIP 图像嵌入器（HTTP 代理方案）
type CLIPImageEmbedder struct {
	BaseURL    string        // ONNX 服务地址（如 http://localhost:8000）
	HTTPClient *http.Client
	Timeout    time.Duration
}

// CLIPImageEmbedderConfig CLIP 嵌入器配置
type CLIPImageEmbedderConfig struct {
	BaseURL string        `json:"base_url"`
	Timeout time.Duration `json:"timeout"`
}

// DefaultCLIPImageEmbedderConfig 返回默认配置
func DefaultCLIPImageEmbedderConfig() CLIPImageEmbedderConfig {
	return CLIPImageEmbedderConfig{
		BaseURL: "http://localhost:8000",
		Timeout: 30 * time.Second,
	}
}

// NewCLIPImageEmbedder 创建 CLIP 图像嵌入器
func NewCLIPImageEmbedder(config CLIPImageEmbedderConfig) *CLIPImageEmbedder {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	return &CLIPImageEmbedder{
		BaseURL: config.BaseURL,
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
		Timeout: config.Timeout,
	}
}

// EmbedImageRequest 图像嵌入请求
type EmbedImageRequest struct {
	ImageData []float32 `json:"image_data"` // NCHW 格式 [1, 3, 224, 224]
	ModelName string    `json:"model_name,omitempty"`
}

// EmbedImageResponse 图像嵌入响应
type EmbedImageResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
	Dim       int       `json:"dim"`
}

// EmbedImage 嵌入图像
// 输入: 预处理后的图像数据（NCHW 格式）
// 输出: 图像嵌入向量
func (e *CLIPImageEmbedder) EmbedImage(imageData []float32) ([]float32, error) {
	req := EmbedImageRequest{
		ImageData: imageData,
		ModelName: "clip-vit-base-patch32",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("onnx: marshal embed request: %w", err)
	}

	url := e.BaseURL + "/embed/image"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("onnx: create embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("onnx: embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("onnx: embed failed with status %d: %s", resp.StatusCode, string(body))
	}

	var embedResp EmbedImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("onnx: decode embed response: %w", err)
	}

	return embedResp.Embedding, nil
}

// EmbedImageFromBytes 从原始图像字节嵌入（完整管道）
func (e *CLIPImageEmbedder) EmbedImageFromBytes(imageBytes []byte) ([]float32, error) {
	// 1. 预处理
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())
	preprocessed, err := preprocessor.Preprocess(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("onnx: preprocess image: %w", err)
	}

	// 2. 嵌入
	return e.EmbedImage(preprocessed)
}

// BatchEmbedImages 批量嵌入图像
func (e *CLIPImageEmbedder) BatchEmbedImages(imageDataList [][]float32) ([][]float32, error) {
	results := make([][]float32, len(imageDataList))
	for i, data := range imageDataList {
		embedding, err := e.EmbedImage(data)
		if err != nil {
			return nil, fmt.Errorf("onnx: batch embed image %d: %w", i, err)
		}
		results[i] = embedding
	}
	return results, nil
}

// GetModelInfo 获取模型信息
func (e *CLIPImageEmbedder) GetModelInfo() (map[string]any, error) {
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

// HealthCheck 健康检查
func (e *CLIPImageEmbedder) HealthCheck() error {
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

// CLIPTextEmbedder CLIP 文本嵌入器（可选，用于跨模态对齐）
type CLIPTextEmbedder struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// EmbedTextRequest 文本嵌入请求
type EmbedTextRequest struct {
	Text      string `json:"text"`
	ModelName string `json:"model_name,omitempty"`
}

// NewCLIPTextEmbedder 创建 CLIP 文本嵌入器
func NewCLIPTextEmbedder(config CLIPImageEmbedderConfig) *CLIPTextEmbedder {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	return &CLIPTextEmbedder{
		BaseURL: config.BaseURL,
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
		Timeout: config.Timeout,
	}
}

// EmbedText 嵌入文本
func (e *CLIPTextEmbedder) EmbedText(text string) ([]float32, error) {
	req := EmbedTextRequest{
		Text:      text,
		ModelName: "clip-vit-base-patch32",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("onnx: marshal text embed request: %w", err)
	}

	url := e.BaseURL + "/embed/text"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("onnx: create text embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("onnx: text embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("onnx: text embed failed with status %d: %s", resp.StatusCode, string(body))
	}

	var embedResp EmbedImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("onnx: decode text embed response: %w", err)
	}

	return embedResp.Embedding, nil
}

// CrossModalSimilarity 计算跨模态相似度（图像-文本）
func CrossModalSimilarity(imageEmbedding, textEmbedding []float32) (float32, error) {
	if len(imageEmbedding) != len(textEmbedding) {
		return 0, fmt.Errorf("onnx: embedding dimensions mismatch: image=%d, text=%d",
			len(imageEmbedding), len(textEmbedding))
	}

	var dotProduct, imageNorm, textNorm float32
	for i := 0; i < len(imageEmbedding); i++ {
		dotProduct += imageEmbedding[i] * textEmbedding[i]
		imageNorm += imageEmbedding[i] * imageEmbedding[i]
		textNorm += textEmbedding[i] * textEmbedding[i]
	}

	if imageNorm == 0 || textNorm == 0 {
		return 0, fmt.Errorf("onnx: zero norm embedding")
	}

	cosineSim := dotProduct / (float32Sqrt(imageNorm) * float32Sqrt(textNorm))
	return cosineSim, nil
}

func float32Sqrt(x float32) float32 {
	// 简单的牛顿迭代法求平方根
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
