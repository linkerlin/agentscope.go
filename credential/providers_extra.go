package credential

import (
	"fmt"

	"github.com/google/uuid"
)

// --- DashScope ---

type DashScopeCredential struct {
	Base
	APIKey string `json:"api_key"`
}

func (c *DashScopeCredential) Provider() string { return "dashscope" }

func (c *DashScopeCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	return data
}

func (c *DashScopeCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeDashScope
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if c.APIKey == "" {
		return fmt.Errorf("dashscope credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewDashScope(name, apiKey string) *DashScopeCredential {
	return &DashScopeCredential{
		Base:   NewBase(uuid.New().String(), name, TypeDashScope),
		APIKey: apiKey,
	}
}

// --- DeepSeek ---

type DeepSeekCredential struct {
	Base
	APIKey string `json:"api_key"`
}

func (c *DeepSeekCredential) Provider() string { return "deepseek" }

func (c *DeepSeekCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	return data
}

func (c *DeepSeekCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeDeepSeek
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if c.APIKey == "" {
		return fmt.Errorf("deepseek credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewDeepSeek(name, apiKey string) *DeepSeekCredential {
	return &DeepSeekCredential{
		Base:   NewBase(uuid.New().String(), name, TypeDeepSeek),
		APIKey: apiKey,
	}
}

// --- Moonshot ---

type MoonshotCredential struct {
	Base
	APIKey string `json:"api_key"`
}

func (c *MoonshotCredential) Provider() string { return "moonshot" }

func (c *MoonshotCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	return data
}

func (c *MoonshotCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeMoonshot
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if c.APIKey == "" {
		return fmt.Errorf("moonshot credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewMoonshot(name, apiKey string) *MoonshotCredential {
	return &MoonshotCredential{
		Base:   NewBase(uuid.New().String(), name, TypeMoonshot),
		APIKey: apiKey,
	}
}

// --- xAI ---

type XAICredential struct {
	Base
	APIKey string `json:"api_key"`
}

func (c *XAICredential) Provider() string { return "xai" }

func (c *XAICredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	return data
}

func (c *XAICredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeXAI
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if c.APIKey == "" {
		return fmt.Errorf("xai credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewXAI(name, apiKey string) *XAICredential {
	return &XAICredential{
		Base:   NewBase(uuid.New().String(), name, TypeXAI),
		APIKey: apiKey,
	}
}

// --- Ollama (local, no API key required) ---

type OllamaCredential struct {
	Base
	BaseURL string `json:"base_url"`
}

func (c *OllamaCredential) Provider() string { return "ollama" }

func (c *OllamaCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["base_url"] = c.BaseURL
	return data
}

func (c *OllamaCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeOllama
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["base_url"].(string); ok {
		c.BaseURL = v
	}
	if c.BaseURL == "" {
		c.BaseURL = "http://localhost:11434"
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewOllama(name string) *OllamaCredential {
	return &OllamaCredential{
		Base:    NewBase(uuid.New().String(), name, TypeOllama),
		BaseURL: "http://localhost:11434",
	}
}

// --- OpenAI Response (Responses API) ---

type OpenAIResponseCredential struct {
	Base
	APIKey       string `json:"api_key"`
	Organization string `json:"organization,omitempty"`
}

func (c *OpenAIResponseCredential) Provider() string { return "openai_response" }

func (c *OpenAIResponseCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	if c.Organization != "" {
		data["organization"] = c.Organization
	}
	return data
}

func (c *OpenAIResponseCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeOpenAIResp
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if v, ok := data["organization"].(string); ok {
		c.Organization = v
	}
	if c.APIKey == "" {
		return fmt.Errorf("openai_response credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewOpenAIResponse(name, apiKey string) *OpenAIResponseCredential {
	return &OpenAIResponseCredential{
		Base:   NewBase(uuid.New().String(), name, TypeOpenAIResp),
		APIKey: apiKey,
	}
}

// --- vLLM ---

type VLLMCredential struct {
	Base
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

func (c *VLLMCredential) Provider() string { return "vllm" }

func (c *VLLMCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	data["base_url"] = c.BaseURL
	return data
}

func (c *VLLMCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeVLLM
	}
	c.Base = NewBase(id, name, typ)
	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if v, ok := data["base_url"].(string); ok {
		c.BaseURL = v
	}
	if c.BaseURL == "" {
		c.BaseURL = "http://localhost:8000"
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewVLLM(name, baseURL string) *VLLMCredential {
	return &VLLMCredential{
		Base:    NewBase(uuid.New().String(), name, TypeVLLM),
		BaseURL: baseURL,
	}
}
