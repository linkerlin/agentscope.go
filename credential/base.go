package credential

import (
	"encoding/json"
	"fmt"
)

// Type is the discriminator for credential kinds (matches Python "type" Literal).
type Type string

const (
	TypeOpenAI     Type = "openai"
	TypeAnthropic  Type = "anthropic"
	TypeGemini     Type = "gemini"
	TypeDashScope  Type = "dashscope"
	TypeDeepSeek   Type = "deepseek"
	TypeMoonshot   Type = "moonshot"
	TypeXAI        Type = "xai"
	TypeOllama     Type = "ollama"
	TypeOpenAIResp Type = "openai_response" // for Responses API creds if needed
	TypeVLLM       Type = "vllm"
)

// Credential is the interface implemented by all typed credentials.
// It supports serialization to a map (for storage in service.Credential.Data or similar)
// and provides the provider key used by AgentFactory / model builders.
type Credential interface {
	// Type returns the discriminator (e.g. "openai").
	Type() Type

	// Name returns a user-facing display name.
	Name() string

	// ID returns the credential identifier (often uuid hex).
	ID() string

	// ToData returns a serializable map representation suitable for JSON storage.
	// Sensitive fields (api_key etc.) are included in plain text here; callers
	// are responsible for encrypting via Cipher before persisting (see Encrypt helpers).
	ToData() map[string]any

	// Provider returns the logical provider key used to select a model builder
	// (e.g. "openai", "anthropic"). Usually matches Type but can differ.
	Provider() string
}

// Base provides common fields and methods for concrete credentials.
type Base struct {
	id   string
	name string
	typ  Type
}

func (b *Base) ID() string   { return b.id }
func (b *Base) Name() string { return b.name }
func (b *Base) Type() Type   { return b.typ }

// MarshalJSON allows Base to participate in JSON roundtrips when embedded.
func (b Base) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"id":   b.id,
		"name": b.name,
		"type": string(b.typ),
	}
	return json.Marshal(m)
}

// UnmarshalBase is a helper for concrete types to populate common fields from data.
func UnmarshalBase(data map[string]any) (id, name string, typ Type, err error) {
	if v, ok := data["id"].(string); ok {
		id = v
	}
	if v, ok := data["name"].(string); ok {
		name = v
	}
	if v, ok := data["type"].(string); ok {
		typ = Type(v)
	}
	if typ == "" {
		// fallback: try "provider" or infer later
		if p, ok := data["provider"].(string); ok {
			typ = Type(p)
		}
	}
	return id, name, typ, nil
}

// ToData is implemented by concrete types; Base provides a minimal version.
func (b *Base) ToData() map[string]any {
	return map[string]any{
		"id":   b.id,
		"name": b.name,
		"type": string(b.typ),
	}
}

// NewBase is for internal use by provider constructors.
func NewBase(id, name string, typ Type) Base {
	return Base{id: id, name: name, typ: typ}
}

// Validate ensures basic fields are present.
func (b *Base) Validate() error {
	if b.id == "" {
		return fmt.Errorf("credential: id is required")
	}
	if b.typ == "" {
		return fmt.Errorf("credential: type is required")
	}
	return nil
}
