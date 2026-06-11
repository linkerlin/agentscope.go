package credential

import (
	"fmt"

	"github.com/google/uuid"
)

// OpenAICredential implements Credential for OpenAI (and compatible) providers.
type OpenAICredential struct {
	Base
	APIKey       string `json:"api_key"`
	Organization string `json:"organization,omitempty"`
	BaseURL      string `json:"base_url,omitempty"`
}

func (c *OpenAICredential) Provider() string { return "openai" }

func (c *OpenAICredential) ToData() map[string]any {
	data := c.Base.ToData()
	data["api_key"] = c.APIKey
	if c.Organization != "" {
		data["organization"] = c.Organization
	}
	if c.BaseURL != "" {
		data["base_url"] = c.BaseURL
	}
	return data
}

func (c *OpenAICredential) fromMap(data map[string]any) error {
	id, name, typ, _ := UnmarshalBase(data)
	if typ == "" {
		typ = TypeOpenAI
	}
	c.Base = NewBase(id, name, typ)

	if v, ok := data["api_key"].(string); ok {
		c.APIKey = v
	}
	if v, ok := data["organization"].(string); ok {
		c.Organization = v
	}
	if v, ok := data["base_url"].(string); ok {
		c.BaseURL = v
	}

	if c.APIKey == "" {
		return fmt.Errorf("openai credential: api_key is required")
	}
	if c.ID() == "" {
		c.Base = NewBase(uuid.New().String(), c.Name(), c.Type())
	}
	return c.Base.Validate()
}

// NewOpenAI creates a new OpenAI credential (convenience).
func NewOpenAI(name, apiKey string) *OpenAICredential {
	return &OpenAICredential{
		Base:   NewBase(uuid.New().String(), name, TypeOpenAI),
		APIKey: apiKey,
	}
}
