package config

import (
	"encoding/json"
	"os"
	"time"
)

// AgentConfig 演进方案中的统一配置快照（可序列化）
type AgentConfig struct {
	Name          string `json:"name"`
	SystemPrompt  string `json:"system_prompt"`
	MaxIterations int    `json:"max_iterations"`

	Model   ModelConfig   `json:"model"`
	Memory  MemoryConfig  `json:"memory"`
	ReMe    *ReMeMemoryConfig `json:"reme,omitempty" yaml:"reme,omitempty"`
	Toolkit ToolkitConfig `json:"toolkit"`
}

// ModelConfig 模型连接
type ModelConfig struct {
	Provider    string  `json:"provider"`
	ModelName   string  `json:"model_name"`
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	Retry       struct {
		MaxAttempts int           `json:"max_attempts"`
		BackoffMs   int           `json:"backoff_ms"`
		Backoff     time.Duration `json:"-"`
	} `json:"retry"`
}

// MemoryConfig 内存策略
type MemoryConfig struct {
	Type        string `json:"type"`
	MaxMessages int    `json:"max_messages"`
	MaxTokens   int    `json:"max_tokens"`
}

// ToolkitConfig 工具执行
type ToolkitConfig struct {
	Parallel    bool `json:"parallel"`
	MaxParallel int  `json:"max_parallel"`
	TimeoutMs   int  `json:"timeout_ms"`
	MaxRetries    int  `json:"max_retries"`
}

// LoadFromFile 从 JSON 文件加载
func LoadFromFile(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c AgentConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
