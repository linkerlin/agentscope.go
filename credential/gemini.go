package credential

import (
	"fmt"

	"github.com/google/uuid"
)

// GeminiCredential for Google Gemini.
type GeminiCredential struct {
	Base
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

func (c *GeminiCredential) Provider() string { return "gemini" }

func (c *GeminiCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	if c.BaseURL != "" {
		data["base_url"] = c.BaseURL
	}
	return data
}

func (c *GeminiCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeGemini
	}
	c.Base = NewBase(id, name, typ)

	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if v, ok := data["base_url"].(string); ok {
		c.BaseURL = v
	}

	if c.APIKey == "" {
		return fmt.Errorf("gemini credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

func NewGemini(name, apiKey string) *GeminiCredential {
	return &GeminiCredential{
		Base:   NewBase(uuid.New().String(), name, TypeGemini),
		APIKey: apiKey,
	}
}
