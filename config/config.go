package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfig 演进方案中的统一配置快照（可序列化）
type AgentConfig struct {
	Name          string `json:"name" yaml:"name"`
	SystemPrompt  string `json:"system_prompt" yaml:"system_prompt"`
	MaxIterations int    `json:"max_iterations" yaml:"max_iterations"`

	Model   ModelConfig       `json:"model" yaml:"model"`
	Memory  MemoryConfig      `json:"memory" yaml:"memory"`
	ReMe    *ReMeMemoryConfig `json:"reme,omitempty" yaml:"reme,omitempty"`
	Toolkit ToolkitConfig     `json:"toolkit" yaml:"toolkit"`
}

// ModelConfig 模型连接
type ModelConfig struct {
	Provider    string  `json:"provider" yaml:"provider"`
	ModelName   string  `json:"model_name" yaml:"model_name"`
	APIKey      string  `json:"api_key" yaml:"api_key"`
	BaseURL     string  `json:"base_url" yaml:"base_url"`
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
	Temperature float64 `json:"temperature" yaml:"temperature"`
	Retry       struct {
		MaxAttempts int           `json:"max_attempts" yaml:"max_attempts"`
		BackoffMs   int           `json:"backoff_ms" yaml:"backoff_ms"`
		Backoff     time.Duration `json:"-" yaml:"-"`
	} `json:"retry" yaml:"retry"`
}

// MemoryConfig 内存策略
type MemoryConfig struct {
	Type        string `json:"type" yaml:"type"`
	MaxMessages int    `json:"max_messages" yaml:"max_messages"`
	MaxTokens   int    `json:"max_tokens" yaml:"max_tokens"`
}

// ToolkitConfig 工具执行
type ToolkitConfig struct {
	Parallel    bool `json:"parallel" yaml:"parallel"`
	MaxParallel int  `json:"max_parallel" yaml:"max_parallel"`
	TimeoutMs   int  `json:"timeout_ms" yaml:"timeout_ms"`
	MaxRetries  int  `json:"max_retries" yaml:"max_retries"`
}

// LoadFromFile 根据文件扩展名自动选择 JSON 或 YAML 解析器。
// 支持 .json / .yaml / .yml 扩展名。
func LoadFromFile(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c AgentConfig
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, err
		}
	default:
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
	}
	return &c, nil
}
