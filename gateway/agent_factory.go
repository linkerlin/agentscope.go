package gateway

import (
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/credential"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/anthropic"
	"github.com/linkerlin/agentscope.go/model/dashscope"
	"github.com/linkerlin/agentscope.go/model/deepseek"
	"github.com/linkerlin/agentscope.go/model/gemini"
	"github.com/linkerlin/agentscope.go/model/moonshot"
	"github.com/linkerlin/agentscope.go/model/ollama"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/model/openai_response"
	"github.com/linkerlin/agentscope.go/model/vllm"
	"github.com/linkerlin/agentscope.go/model/xai"
	"github.com/linkerlin/agentscope.go/service"
)

// ModelBuilderFunc builds a ChatModel from an API key, model name and optional base URL.
type ModelBuilderFunc func(apiKey, modelName, baseURL string) (model.ChatModel, error)

// AgentFactory constructs agent instances from persisted AgentConfig and Credential.
type AgentFactory struct {
	modelBuilders map[string]ModelBuilderFunc
	cipher        *service.Cipher
}

// NewAgentFactory creates a factory with all built-in model providers registered.
// Pass a non-nil cipher if credentials are encrypted.
func NewAgentFactory(cipher *service.Cipher) *AgentFactory {
	f := &AgentFactory{
		modelBuilders: make(map[string]ModelBuilderFunc),
		cipher:        cipher,
	}
	f.registerBuiltins()
	return f
}

// RegisterProvider registers a custom model builder for the given provider name.
func (f *AgentFactory) RegisterProvider(name string, fn ModelBuilderFunc) {
	f.modelBuilders[name] = fn
}

// Build creates an agent.Agent from the given configuration and credential.
// It decrypts the credential if a cipher is available, builds the ChatModel,
// and assembles a ReAct agent with the configured system prompt.
func (f *AgentFactory) Build(cfg *service.AgentConfig, cred *service.Credential) (agent.Agent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("agent_factory: agent config is nil")
	}
	if cred == nil {
		return nil, fmt.Errorf("agent_factory: credential is nil")
	}

	apiKey, err := f.decryptKey(cred)
	if err != nil {
		return nil, fmt.Errorf("agent_factory: decrypt credential: %w", err)
	}

	provider, modelName := parseModelID(cfg.ModelID, cred.Provider)
	if provider == "" {
		return nil, fmt.Errorf("agent_factory: cannot determine provider for model %q", cfg.ModelID)
	}

	builder, ok := f.modelBuilders[provider]
	if !ok {
		return nil, fmt.Errorf("agent_factory: unsupported provider %q", provider)
	}

	baseURL := ""
	if cfg.Metadata != nil {
		if u, _ := cfg.Metadata["base_url"].(string); u != "" {
			baseURL = u
		}
	}

	chatModel, err := builder(apiKey, modelName, baseURL)
	if err != nil {
		return nil, fmt.Errorf("agent_factory: build model for %s/%s: %w", provider, modelName, err)
	}

	b := react.Builder().
		Name(cfg.Name).
		ID(cfg.ID).
		SysPrompt(cfg.SystemPrompt).
		Model(chatModel)

	if cfg.Metadata != nil {
		b = b.Metadata(cfg.Metadata)
	}

	return b.Build()
}

// BuildFromTyped builds an agent from config + a typed Credential (plain-text secrets expected inside the typed struct).
// This is the preferred path when you already have a decrypted/typed credential (e.g. after credential.FromServiceCredential).
func (f *AgentFactory) BuildFromTyped(cfg *service.AgentConfig, cred credential.Credential) (agent.Agent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("agent_factory: agent config is nil")
	}
	if cred == nil {
		return nil, fmt.Errorf("agent_factory: credential is nil")
	}

	// Extract api key from common typed credential shapes via ToData (or direct type assertions for speed).
	data := cred.ToData()
	apiKey, _ := data["api_key"].(string)
	if apiKey == "" {
		// fallback for any credential that puts the key under different key
		if k, ok := data["key"].(string); ok {
			apiKey = k
		}
	}
	// Note: apiKey may be empty for providers that don't require one (e.g. Ollama).

	provider := cred.Provider()
	modelName := ""
	if cfg.ModelID != "" {
		_, m := parseModelID(cfg.ModelID, provider)
		modelName = m
	}

	builder, ok := f.modelBuilders[provider]
	if !ok {
		// last chance: try using the Provider() directly as key if registered
		builder, ok = f.modelBuilders[cred.Provider()]
	}
	if !ok {
		return nil, fmt.Errorf("agent_factory: unsupported provider %q from typed credential", provider)
	}

	baseURL := ""
	if cfg.Metadata != nil {
		if u, _ := cfg.Metadata["base_url"].(string); u != "" {
			baseURL = u
		}
	}
	if bu, _ := data["base_url"].(string); baseURL == "" && bu != "" {
		baseURL = bu
	}

	chatModel, err := builder(apiKey, modelName, baseURL)
	if err != nil {
		return nil, fmt.Errorf("agent_factory: build model for %s/%s: %w", provider, modelName, err)
	}

	b := react.Builder().
		Name(cfg.Name).
		ID(cfg.ID).
		SysPrompt(cfg.SystemPrompt).
		Model(chatModel)

	if cfg.Metadata != nil {
		b = b.Metadata(cfg.Metadata)
	}

	return b.Build()
}

// decryptKey decrypts the credential's encrypted field when a cipher is present.
func (f *AgentFactory) decryptKey(cred *service.Credential) (string, error) {
	if f.cipher == nil || cred.Encrypted == "" {
		return cred.Encrypted, nil
	}
	return f.cipher.Decrypt(cred.Encrypted)
}

// parseModelID extracts provider and model name from the ModelID field.
// Supported forms: "provider/modelName", "provider:modelName".
// If no separator is found, provider falls back to credProvider and the
// entire string is treated as the model name.
func parseModelID(modelID, credProvider string) (provider, modelName string) {
	for _, sep := range []string{"/", ":"} {
		if i := strings.Index(modelID, sep); i > 0 {
			return modelID[:i], modelID[i+1:]
		}
	}
	return credProvider, modelID
}

// registerBuiltins registers the built-in model providers.
func (f *AgentFactory) registerBuiltins() {
	f.modelBuilders["openai"] = func(key, name, url string) (model.ChatModel, error) {
		b := openai.Builder().APIKey(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["anthropic"] = func(key, name, url string) (model.ChatModel, error) {
		b := anthropic.NewBuilder().APIKey(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["gemini"] = func(key, name, url string) (model.ChatModel, error) {
		b := gemini.NewBuilder().APIKey(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["deepseek"] = func(key, name, url string) (model.ChatModel, error) {
		b := deepseek.Builder(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["moonshot"] = func(key, name, url string) (model.ChatModel, error) {
		b := moonshot.Builder(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["xai"] = func(key, name, url string) (model.ChatModel, error) {
		b := xai.Builder(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["ollama"] = func(key, name, url string) (model.ChatModel, error) {
		b := ollama.NewBuilder().ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		if key != "" {
			b = b.APIKey(key)
		}
		return b.Build()
	}
	f.modelBuilders["vllm"] = func(key, name, url string) (model.ChatModel, error) {
		b := vllm.Builder(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["dashscope"] = func(key, name, url string) (model.ChatModel, error) {
		b := dashscope.Builder().APIKey(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
	f.modelBuilders["openai_response"] = func(key, name, url string) (model.ChatModel, error) {
		b := openai_response.Builder().APIKey(key).ModelName(name)
		if url != "" {
			b = b.BaseURL(url)
		}
		return b.Build()
	}
}
