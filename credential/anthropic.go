package credential

import (
	"fmt"

	"github.com/google/uuid"
)

// AnthropicCredential implements Credential for Anthropic Claude.
type AnthropicCredential struct {
	Base
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

func (c *AnthropicCredential) Provider() string { return "anthropic" }

func (c *AnthropicCredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	if c.BaseURL != "" {
		data["base_url"] = c.BaseURL
	}
	return data
}

func (c *AnthropicCredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeAnthropic
	}
	c.Base = NewBase(id, name, typ)

	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if v, ok := data["base_url"].(string); ok {
		c.BaseURL = v
	}

	if c.APIKey == "" {
		return fmt.Errorf("anthropic credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

// NewAnthropic creates a new Anthropic credential (convenience).
func NewAnthropic(name, apiKey string) *AnthropicCredential {
	return &AnthropicCredential{
		Base:   NewBase(uuid.New().String(), name, TypeAnthropic),
		APIKey: apiKey,
	}
}
