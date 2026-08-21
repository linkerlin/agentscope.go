// Package llmenv 为示例（cookbook/examples/scripts）提供统一的 LLM API 配置加载：
// 优先读进程环境变量，缺失时回退到 .env 文件（从当前目录向上逐级查找）。
//
// 支持的变量：
//
//	OPENAI_API_KEY   API 密钥（必填）
//	OPENAI_BASE_URL  API 地址（可选，用于 OpenAI 兼容的第三方网关）
//	OPENAI_MODEL     模型名（可选，默认 gpt-4o-mini）
package llmenv

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/linkerlin/agentscope.go/model/openai"
)

// DefaultModel 是未设置 OPENAI_MODEL 时使用的模型名。
const DefaultModel = "gpt-4o-mini"

// Config 是加载后的 LLM 配置。
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

// Load 返回 LLM 配置。真实环境变量优先于 .env 文件；Model 为空时取 DefaultModel。
func Load() Config {
	fileVars := loadDotEnv()
	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileVars[key]
	}
	c := Config{
		APIKey:  get("OPENAI_API_KEY"),
		BaseURL: get("OPENAI_BASE_URL"),
		Model:   get("OPENAI_MODEL"),
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	return c
}

// MustChatModel 用加载的配置构建 OpenAI 兼容 ChatModel，缺少 APIKey 时 log.Fatal。
func MustChatModel() *openai.OpenAIChatModel {
	c := Load()
	if c.APIKey == "" {
		log.Fatal("OPENAI_API_KEY is required (set it in the environment or a .env file)")
	}
	m, err := openai.Builder().APIKey(c.APIKey).BaseURL(c.BaseURL).ModelName(c.Model).Build()
	if err != nil {
		log.Fatal(err)
	}
	return m
}

// loadDotEnv 解析向上查找到的第一个 .env 文件（KEY=VALUE，支持 # 注释、
// 成对引号和 export 前缀）。文件不存在时返回 nil。
func loadDotEnv() map[string]string {
	data, err := os.ReadFile(findDotEnv())
	if err != nil {
		return nil
	}
	vars := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key != "" {
			vars[key] = val
		}
	}
	return vars
}

// findDotEnv 从当前目录向上查找 .env，返回第一个匹配的路径；找不到返回空串。
func findDotEnv() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, ".env")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
