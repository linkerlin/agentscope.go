package onnx

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ModelInfo 模型信息
type ModelInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	URL         string    `json:"url"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ModelCacheEntry 缓存条目
type ModelCacheEntry struct {
	Info       ModelInfo
	LocalPath  string
	Downloaded bool
	LastUsed   time.Time
}

// ModelManager 模型管理器（下载/缓存/版本）
type ModelManager struct {
	CacheDir     string
	Models       map[string]*ModelCacheEntry
	mu           sync.RWMutex
	HTTPClient   *http.Client
	MaxCacheSize int64 // 最大缓存大小（字节）
}

// ModelManagerConfig 模型管理器配置
type ModelManagerConfig struct {
	CacheDir     string        `json:"cache_dir"`
	MaxCacheSize int64         `json:"max_cache_size"`
	Timeout      time.Duration `json:"timeout"`
}

// DefaultModelManagerConfig 返回默认配置
func DefaultModelManagerConfig() ModelManagerConfig {
	homeDir, _ := os.UserHomeDir()
	return ModelManagerConfig{
		CacheDir:     filepath.Join(homeDir, ".agentscope", "onnx_models"),
		MaxCacheSize: 10 * 1024 * 1024 * 1024, // 10GB
		Timeout:      5 * time.Minute,
	}
}

// NewModelManager 创建模型管理器
func NewModelManager(config ModelManagerConfig) (*ModelManager, error) {
	if config.CacheDir == "" {
		config = DefaultModelManagerConfig()
	}

	// 确保缓存目录存在
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("onnx: create cache dir: %w", err)
	}

	return &ModelManager{
		CacheDir:     config.CacheDir,
		Models:       make(map[string]*ModelCacheEntry),
		HTTPClient:   &http.Client{Timeout: config.Timeout},
		MaxCacheSize: config.MaxCacheSize,
	}, nil
}

// RegisterModel 注册模型
func (m *ModelManager) RegisterModel(info ModelInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.modelKey(info.Name, info.Version)
	localPath := filepath.Join(m.CacheDir, fmt.Sprintf("%s_%s.onnx", info.Name, info.Version))

	m.Models[key] = &ModelCacheEntry{
		Info:       info,
		LocalPath:  localPath,
		Downloaded: false,
		LastUsed:   time.Now(),
	}
}

// DownloadModel 下载模型
func (m *ModelManager) DownloadModel(name, version string) error {
	m.mu.Lock()
	entry, exists := m.Models[m.modelKey(name, version)]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("onnx: model %s@%s not registered", name, version)
	}

	if entry.Downloaded {
		m.mu.Unlock()
		return nil // 已下载
	}
	m.mu.Unlock()

	// 下载
	req, err := http.NewRequest("GET", entry.Info.URL, nil)
	if err != nil {
		return fmt.Errorf("onnx: create download request: %w", err)
	}

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("onnx: download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("onnx: download failed with status %d", resp.StatusCode)
	}

	// 写入文件
	file, err := os.Create(entry.LocalPath)
	if err != nil {
		return fmt.Errorf("onnx: create model file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(entry.LocalPath)
		return fmt.Errorf("onnx: write model file: %w", err)
	}

	// 更新状态
	m.mu.Lock()
	entry.Downloaded = true
	entry.LastUsed = time.Now()
	m.mu.Unlock()

	fmt.Printf("onnx: downloaded model %s@%s (%d bytes)\n", name, version, written)
	return nil
}

// GetModelPath 获取模型本地路径
func (m *ModelManager) GetModelPath(name, version string) (string, error) {
	m.mu.RLock()
	entry, exists := m.Models[m.modelKey(name, version)]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("onnx: model %s@%s not found", name, version)
	}

	if !entry.Downloaded {
		// 自动下载
		if err := m.DownloadModel(name, version); err != nil {
			return "", err
		}
	}

	m.mu.Lock()
	entry.LastUsed = time.Now()
	m.mu.Unlock()

	return entry.LocalPath, nil
}

// ListModels 列出所有模型
func (m *ModelManager) ListModels() []ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ModelInfo, 0, len(m.Models))
	for _, entry := range m.Models {
		result = append(result, entry.Info)
	}
	return result
}

// RemoveModel 删除模型
func (m *ModelManager) RemoveModel(name, version string) error {
	m.mu.Lock()
	entry, exists := m.Models[m.modelKey(name, version)]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("onnx: model %s@%s not found", name, version)
	}

	delete(m.Models, m.modelKey(name, version))
	m.mu.Unlock()

	if entry.Downloaded {
		if err := os.Remove(entry.LocalPath); err != nil {
			return fmt.Errorf("onnx: remove model file: %w", err)
		}
	}

	return nil
}

// CleanupCache 清理缓存（LRU 策略）
func (m *ModelManager) CleanupCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var totalSize int64
	for _, entry := range m.Models {
		if entry.Downloaded {
			info, err := os.Stat(entry.LocalPath)
			if err == nil {
				totalSize += info.Size()
			}
		}
	}

	if totalSize <= m.MaxCacheSize {
		return nil
	}

	// 按最后使用时间排序，删除最旧的
	type modelWithTime struct {
		key  string
		time time.Time
		size int64
	}

	models := make([]modelWithTime, 0, len(m.Models))
	for key, entry := range m.Models {
		if entry.Downloaded {
			info, err := os.Stat(entry.LocalPath)
			if err == nil {
				models = append(models, modelWithTime{key, entry.LastUsed, info.Size()})
			}
		}
	}

	// 简单选择排序（按时间）
	for i := 0; i < len(models); i++ {
		for j := i + 1; j < len(models); j++ {
			if models[j].time.Before(models[i].time) {
				models[i], models[j] = models[j], models[i]
			}
		}
	}

	// 删除最旧的直到满足大小限制
	for _, model := range models {
		if totalSize <= m.MaxCacheSize {
			break
		}

		entry := m.Models[model.key]
		if err := os.Remove(entry.LocalPath); err == nil {
			entry.Downloaded = false
			totalSize -= model.size
		}
	}

	return nil
}

// modelKey 生成模型键
func (m *ModelManager) modelKey(name, version string) string {
	return fmt.Sprintf("%s@%s", name, version)
}

// PredefinedModels 预定义模型列表
func PredefinedModels() []ModelInfo {
	return []ModelInfo{
		{
			Name:        "clip-vit-base-patch32",
			Version:     "1.0",
			URL:         "https://huggingface.co/openai/clip-vit-base-patch32/resolve/main/onnx/model.onnx",
			Size:        300 * 1024 * 1024, // ~300MB
			Description: "CLIP ViT-B/32 图像-文本对齐模型",
		},
		{
			Name:        "clip-vit-base-patch16",
			Version:     "1.0",
			URL:         "https://huggingface.co/openai/clip-vit-base-patch16/resolve/main/onnx/model.onnx",
			Size:        600 * 1024 * 1024, // ~600MB
			Description: "CLIP ViT-B/16 图像-文本对齐模型",
		},
		{
			Name:        "whisper-base",
			Version:     "1.0",
			URL:         "https://huggingface.co/openai/whisper-base/resolve/main/onnx/model.onnx",
			Size:        150 * 1024 * 1024, // ~150MB
			Description: "Whisper Base 语音识别模型",
		},
		{
			Name:        "whisper-small",
			Version:     "1.0",
			URL:         "https://huggingface.co/openai/whisper-small/resolve/main/onnx/model.onnx",
			Size:        500 * 1024 * 1024, // ~500MB
			Description: "Whisper Small 语音识别模型",
		},
	}
}
