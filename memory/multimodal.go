package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/model"
)

// MultimodalContentType 多模态内容类型
type MultimodalContentType string

const (
	ContentTypeText  MultimodalContentType = "text"
	ContentTypeImage MultimodalContentType = "image"
	ContentTypeAudio MultimodalContentType = "audio"
	ContentTypeVideo MultimodalContentType = "video"
	ContentTypeFile  MultimodalContentType = "file"
)

// MultimodalContent 多模态内容块
type MultimodalContent struct {
	Type     MultimodalContentType `json:"type"`
	Text     string                `json:"text,omitempty"`
	ImageURL string                `json:"image_url,omitempty"`
	AudioURL string                `json:"audio_url,omitempty"`
	VideoURL string                `json:"video_url,omitempty"`
	FileURL  string                `json:"file_url,omitempty"`
	Base64   string                `json:"base64,omitempty"`
	Metadata map[string]any        `json:"metadata,omitempty"`
}

// MultimodalEmbeddingModel 多模态嵌入模型接口
type MultimodalEmbeddingModel interface {
	EmbeddingModel
	// EmbedImage 嵌入图像（URL 或 base64）
	EmbedImage(ctx context.Context, imageURL string, base64 string) ([]float32, error)
	// EmbedAudio 嵌入音频（URL 或 base64）
	EmbedAudio(ctx context.Context, audioURL string, base64 string) ([]float32, error)
	// EmbedVideo 嵌入视频（URL）
	EmbedVideo(ctx context.Context, videoURL string) ([]float32, error)
	// EmbedMultimodal 嵌入多模态内容列表
	EmbedMultimodal(ctx context.Context, contents []MultimodalContent) ([]float32, error)
}

// ImageEmbeddingModel 图像专用嵌入模型
type ImageEmbeddingModel interface {
	// EmbedImage 嵌入图像
	EmbedImage(ctx context.Context, imageURL string, base64 string) ([]float32, error)
	// EmbedImageBatch 批量嵌入图像
	EmbedImageBatch(ctx context.Context, imageURLs []string) ([][]float32, error)
	// Dimension 返回嵌入维度
	Dimension() int
}

// AudioEmbeddingModel 音频专用嵌入模型
type AudioEmbeddingModel interface {
	// EmbedAudio 嵌入音频
	EmbedAudio(ctx context.Context, audioURL string, base64 string) ([]float32, error)
	// EmbedAudioBatch 批量嵌入音频
	EmbedAudioBatch(ctx context.Context, audioURLs []string) ([][]float32, error)
	// Dimension 返回嵌入维度
	Dimension() int
}

// OpenAIVisionEmbedding OpenAI Vision 多模态嵌入（通过 GPT-4V 描述 + 文本嵌入）
type OpenAIVisionEmbedding struct {
	chatModel model.ChatModel
	embed     EmbeddingModel
}

// NewOpenAIVisionEmbedding 创建 OpenAI Vision 多模态嵌入器
func NewOpenAIVisionEmbedding(chatModel model.ChatModel, embed EmbeddingModel) *OpenAIVisionEmbedding {
	return &OpenAIVisionEmbedding{chatModel: chatModel, embed: embed}
}

// Embed 实现 EmbeddingModel 接口（文本）
func (o *OpenAIVisionEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	return o.embed.Embed(ctx, text)
}

// EmbedBatch 批量嵌入文本
func (o *OpenAIVisionEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return o.embed.EmbedBatch(ctx, texts)
}

// EmbedImage 嵌入图像：先通过 GPT-4V 生成描述，再嵌入描述文本
func (o *OpenAIVisionEmbedding) EmbedImage(ctx context.Context, imageURL string, base64 string) ([]float32, error) {
	// 简化实现：使用图像 URL/文件名作为代理文本
	// 实际生产环境应调用 GPT-4V API 生成描述
	proxyText := fmt.Sprintf("image content from %s", imageURL)
	if imageURL == "" {
		proxyText = "image content"
	}
	return o.embed.Embed(ctx, proxyText)
}

// EmbedAudio 嵌入音频：通过 Whisper 转录后嵌入
func (o *OpenAIVisionEmbedding) EmbedAudio(ctx context.Context, audioURL string, base64 string) ([]float32, error) {
	// 简化实现：使用音频 URL/文件名作为代理文本
	// 实际生产环境应调用 Whisper API 转录
	proxyText := fmt.Sprintf("audio content from %s", audioURL)
	if audioURL == "" {
		proxyText = "audio content"
	}
	return o.embed.Embed(ctx, proxyText)
}

// EmbedVideo 嵌入视频：提取关键帧描述后嵌入
func (o *OpenAIVisionEmbedding) EmbedVideo(ctx context.Context, videoURL string) ([]float32, error) {
	// 简化实现：使用视频 URL/文件名作为代理文本
	// 实际生产环境应提取关键帧 + 音频摘要
	proxyText := fmt.Sprintf("video content from %s", videoURL)
	return o.embed.Embed(ctx, proxyText)
}

// EmbedMultimodal 嵌入多模态内容列表
func (o *OpenAIVisionEmbedding) EmbedMultimodal(ctx context.Context, contents []MultimodalContent) ([]float32, error) {
	var combinedText string
	for _, c := range contents {
		switch c.Type {
		case ContentTypeText:
			combinedText += c.Text + " "
		case ContentTypeImage:
			// 生成图像描述
			desc, err := o.EmbedImage(ctx, c.ImageURL, c.Base64)
			if err == nil {
				_ = desc
				combinedText += "[image] "
			}
		case ContentTypeAudio:
			combinedText += "[audio] "
		case ContentTypeVideo:
			combinedText += "[video] "
		}
	}
	return o.embed.Embed(ctx, combinedText)
}

// CLIPImageEmbedding CLIP 风格图像嵌入（占位实现，实际需 ONNX Runtime）
type CLIPImageEmbedding struct {
	dim int
	mu  sync.RWMutex
	// 实际实现需要 ONNX Runtime 或 TensorFlow Lite
}

// NewCLIPImageEmbedding 创建 CLIP 图像嵌入器
func NewCLIPImageEmbedding(dim int) *CLIPImageEmbedding {
	return &CLIPImageEmbedding{dim: dim}
}

// EmbedImage 嵌入图像（占位实现）
func (c *CLIPImageEmbedding) EmbedImage(ctx context.Context, imageURL string, base64 string) ([]float32, error) {
	// 占位实现：返回随机向量
	// 实际生产环境应加载 CLIP 模型并通过 ONNX Runtime 推理
	c.mu.RLock()
	defer c.mu.RUnlock()

	vec := make([]float32, c.dim)
	for i := range vec {
		vec[i] = float32(i) / float32(c.dim) // 占位值
	}
	return vec, nil
}

// EmbedImageBatch 批量嵌入图像
func (c *CLIPImageEmbedding) EmbedImageBatch(ctx context.Context, imageURLs []string) ([][]float32, error) {
	results := make([][]float32, len(imageURLs))
	for i, url := range imageURLs {
		vec, err := c.EmbedImage(ctx, url, "")
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

// Dimension 返回嵌入维度
func (c *CLIPImageEmbedding) Dimension() int {
	return c.dim
}

// WhisperAudioEmbedding Whisper 风格音频嵌入（占位实现）
type WhisperAudioEmbedding struct {
	dim int
	mu  sync.RWMutex
}

// NewWhisperAudioEmbedding 创建 Whisper 音频嵌入器
func NewWhisperAudioEmbedding(dim int) *WhisperAudioEmbedding {
	return &WhisperAudioEmbedding{dim: dim}
}

// EmbedAudio 嵌入音频（占位实现）
func (w *WhisperAudioEmbedding) EmbedAudio(ctx context.Context, audioURL string, base64 string) ([]float32, error) {
	// 占位实现：返回基于音频时长/文件名的代理向量
	// 实际生产环境应调用 Whisper API 获取嵌入
	w.mu.RLock()
	defer w.mu.RUnlock()

	vec := make([]float32, w.dim)
	for i := range vec {
		vec[i] = float32(i) / float32(w.dim) // 占位值
	}
	return vec, nil
}

// EmbedAudioBatch 批量嵌入音频
func (w *WhisperAudioEmbedding) EmbedAudioBatch(ctx context.Context, audioURLs []string) ([][]float32, error) {
	results := make([][]float32, len(audioURLs))
	for i, url := range audioURLs {
		vec, err := w.EmbedAudio(ctx, url, "")
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

// Dimension 返回嵌入维度
func (w *WhisperAudioEmbedding) Dimension() int {
	return w.dim
}

// MultimodalMemoryNode 多模态记忆节点
type MultimodalMemoryNode struct {
	MemoryNode
	Contents []MultimodalContent `json:"contents"` // 多模态内容列表
}

// NewMultimodalMemoryNode 创建多模态记忆节点
func NewMultimodalMemoryNode(memType MemoryType, target string, contents []MultimodalContent) *MultimodalMemoryNode {
	// 合并所有文本内容
	var textParts []string
	for _, c := range contents {
		if c.Type == ContentTypeText {
			textParts = append(textParts, c.Text)
		}
	}

	node := NewMemoryNode(memType, target, strings.Join(textParts, " "))
	return &MultimodalMemoryNode{
		MemoryNode: *node,
		Contents:   contents,
	}
}

// TokenCount 多模态 Token 计数
// 文本：精确计数
// 图像：base64 按 len(data)//4，URL 按字符串
// 音频：按时长估算
func (m *MultimodalMemoryNode) TokenCount() int {
	var count int
	for _, c := range m.Contents {
		switch c.Type {
		case ContentTypeText:
			count += len(c.Text) / 4 // 简化估算：1 token ≈ 4 字符
		case ContentTypeImage:
			if c.Base64 != "" {
				count += len(c.Base64) / 4 // base64 编码长度
			} else if c.ImageURL != "" {
				count += len(c.ImageURL) // URL 作为字符串
			}
		case ContentTypeAudio:
			// 按音频时长估算（假设 1 秒 ≈ 10 tokens）
			if duration, ok := c.Metadata["duration"].(float64); ok {
				count += int(duration * 10)
			} else {
				count += 100 // 默认估算
			}
		case ContentTypeVideo:
			// 按视频时长估算（假设 1 秒 ≈ 5 tokens）
			if duration, ok := c.Metadata["duration"].(float64); ok {
				count += int(duration * 5)
			} else {
				count += 500 // 默认估算
			}
		}
	}
	return count
}
