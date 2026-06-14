package credential

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Factory manages registration and (de)serialization of Credential implementations.
// It is the Go counterpart to Python's CredentialFactory (with list_schemas for dynamic UIs).
type Factory struct {
	mu      sync.RWMutex
	creds   map[Type]func(map[string]any) (Credential, error)
	schemas map[Type]map[string]any // pre-registered JSON schema fragments (title + properties style)
}

var DefaultFactory = NewFactory()

// NewFactory creates a factory with built-in providers pre-registered.
func NewFactory() *Factory {
	f := &Factory{
		creds:   make(map[Type]func(map[string]any) (Credential, error)),
		schemas: make(map[Type]map[string]any),
	}
	f.registerBuiltins()
	return f
}

func (f *Factory) registerBuiltins() {
	// OpenAI
	f.Register(TypeOpenAI, func(data map[string]any) (Credential, error) {
		c := &OpenAICredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, openAISchema())

	// Anthropic
	f.Register(TypeAnthropic, func(data map[string]any) (Credential, error) {
		c := &AnthropicCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, anthropicSchema())

	// Gemini
	f.Register(TypeGemini, func(data map[string]any) (Credential, error) {
		c := &GeminiCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, geminiSchema())

	// DashScope
	f.Register(TypeDashScope, func(data map[string]any) (Credential, error) {
		c := &DashScopeCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, apiKeySchema("DashScope API", "dashscope"))

	// DeepSeek
	f.Register(TypeDeepSeek, func(data map[string]any) (Credential, error) {
		c := &DeepSeekCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, apiKeySchema("DeepSeek API", "deepseek"))

	// Moonshot
	f.Register(TypeMoonshot, func(data map[string]any) (Credential, error) {
		c := &MoonshotCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, apiKeySchema("Moonshot API", "moonshot"))

	// xAI
	f.Register(TypeXAI, func(data map[string]any) (Credential, error) {
		c := &XAICredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, apiKeySchema("xAI (Grok) API", "xai"))

	// Ollama
	f.Register(TypeOllama, func(data map[string]any) (Credential, error) {
		c := &OllamaCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, ollamaSchema())

	// OpenAI Response
	f.Register(TypeOpenAIResp, func(data map[string]any) (Credential, error) {
		c := &OpenAIResponseCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, apiKeySchema("OpenAI Responses API", "openai_response"))

	// vLLM
	f.Register(TypeVLLM, func(data map[string]any) (Credential, error) {
		c := &VLLMCredential{}
		if err := c.fromMap(data); err != nil {
			return nil, err
		}
		return c, nil
	}, vllmSchema())
}

// Register adds or overrides a credential type constructor and its schema.
func (f *Factory) Register(typ Type, ctor func(map[string]any) (Credential, error), schema map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creds[typ] = ctor
	if schema != nil {
		f.schemas[typ] = schema
	}
}

// FromMap deserializes a map (e.g. from service.Credential.Data or JSON body) into a typed Credential.
func (f *Factory) FromMap(data map[string]any) (Credential, error) {
	if data == nil {
		return nil, fmt.Errorf("credential: nil data")
	}

	typVal, _ := data["type"].(string)
	if typVal == "" {
		// try common alternatives
		if p, ok := data["provider"].(string); ok {
			typVal = p
		}
	}
	typ := Type(typVal)

	f.mu.RLock()
	ctor, ok := f.creds[typ]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("credential: unknown type %q", typ)
	}
	return ctor(data)
}

// FromJSON is a convenience wrapper around FromMap.
func (f *Factory) FromJSON(b []byte) (Credential, error) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("credential: unmarshal: %w", err)
	}
	return f.FromMap(m)
}

// ListSchemas returns schemas for all registered types (shape compatible with Python list_schemas + frontend CredentialSchema).
// Each entry has at least "title", "type", "properties".
func (f *Factory) ListSchemas() []map[string]any {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := make([]map[string]any, 0, len(f.schemas))
	for _, s := range f.schemas {
		if s != nil {
			out = append(out, s)
		}
	}
	return out
}

// GetSchema returns the schema for a specific type (or nil).
func (f *Factory) GetSchema(typ Type) map[string]any {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.schemas[typ]
}

// --- Built-in schemas (simple but sufficient for dynamic forms) ---

func openAISchema() map[string]any {
	return map[string]any{
		"title": "OpenAI API",
		"type":  "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "string", "description": "Credential id"},
			"name": map[string]any{"type": "string", "description": "Display name"},
			"type": map[string]any{"type": "string", "const": "openai"},
			"api_key": map[string]any{
				"type":        "string",
				"format":      "password",
				"writeOnly":   true,
				"description": "OpenAI API key",
			},
			"organization": map[string]any{
				"type":        "string",
				"description": "Optional organization ID",
			},
			"base_url": map[string]any{
				"type":        "string",
				"description": "Base URL for OpenAI-compatible endpoints",
			},
		},
		"required": []string{"api_key"},
	}
}

func anthropicSchema() map[string]any {
	return map[string]any{
		"title": "Anthropic API",
		"type":  "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
			"type": map[string]any{"type": "string", "const": "anthropic"},
			"api_key": map[string]any{
				"type":        "string",
				"format":      "password",
				"writeOnly":   true,
				"description": "Anthropic API key",
			},
			"base_url": map[string]any{
				"type":        "string",
				"description": "Optional base URL override",
			},
		},
		"required": []string{"api_key"},
	}
}

func geminiSchema() map[string]any {
	return map[string]any{
		"title": "Google Gemini API",
		"type":  "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
			"type": map[string]any{"type": "string", "const": "gemini"},
			"api_key": map[string]any{
				"type":        "string",
				"format":      "password",
				"writeOnly":   true,
				"description": "Gemini API key",
			},
			"base_url": map[string]any{
				"type":        "string",
				"description": "Optional base URL override",
			},
		},
		"required": []string{"api_key"},
	}
}

func apiKeySchema(title, typ string) map[string]any {
	return map[string]any{
		"title": title,
		"type":  "object",
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"name":    map[string]any{"type": "string"},
			"type":    map[string]any{"type": "string", "const": typ},
			"api_key": map[string]any{"type": "string", "format": "password", "writeOnly": true},
		},
		"required": []string{"api_key"},
	}
}

func ollamaSchema() map[string]any {
	return map[string]any{
		"title": "Ollama (Local)",
		"type":  "object",
		"properties": map[string]any{
			"id":       map[string]any{"type": "string"},
			"name":     map[string]any{"type": "string"},
			"type":     map[string]any{"type": "string", "const": "ollama"},
			"base_url": map[string]any{"type": "string", "description": "Ollama server URL", "default": "http://localhost:11434"},
		},
	}
}

func vllmSchema() map[string]any {
	return map[string]any{
		"title": "vLLM",
		"type":  "object",
		"properties": map[string]any{
			"id":       map[string]any{"type": "string"},
			"name":     map[string]any{"type": "string"},
			"type":     map[string]any{"type": "string", "const": "vllm"},
			"api_key":  map[string]any{"type": "string", "format": "password", "writeOnly": true},
			"base_url": map[string]any{"type": "string", "description": "vLLM server URL", "default": "http://localhost:8000"},
		},
	}
}
