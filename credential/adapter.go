package credential

import (
	"encoding/json"
	"fmt"

	"github.com/linkerlin/agentscope.go/service"
)

// EncryptToService converts a typed Credential into a storable service.Credential.
// It JSON-marshals the credential's data (including secrets) and encrypts it using the provided cipher.
// Provider and Label are populated for compatibility with existing code.
func EncryptToService(c Credential, cipher *service.Cipher) (*service.Credential, error) {
	if c == nil {
		return nil, fmt.Errorf("credential: nil credential")
	}
	data := c.ToData()

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("credential: marshal data: %w", err)
	}

	var enc string
	if cipher != nil {
		enc, err = cipher.Encrypt(string(raw))
		if err != nil {
			return nil, fmt.Errorf("credential: encrypt: %w", err)
		}
	} else {
		// In dev / no-cipher scenarios we store plaintext (not recommended for prod).
		enc = string(raw)
	}

	sc := &service.Credential{
		ID:       c.ID(),
		UserID:   "", // caller should set
		Provider: c.Provider(),
		Label:    c.Name(),
		Encrypted: enc,
	}
	return sc, nil
}

// DecryptFromService reconstructs a typed Credential from a stored service.Credential using the cipher.
func DecryptFromService(sc *service.Credential, cipher *service.Cipher, f *Factory) (Credential, error) {
	if sc == nil {
		return nil, fmt.Errorf("credential: nil service credential")
	}
	if f == nil {
		f = DefaultFactory
	}

	var plaintext string
	var err error
	if cipher != nil && sc.Encrypted != "" {
		plaintext, err = cipher.Decrypt(sc.Encrypted)
		if err != nil {
			return nil, fmt.Errorf("credential: decrypt: %w", err)
		}
	} else {
		plaintext = sc.Encrypted // may be plaintext in dev
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(plaintext), &data); err != nil {
		// Fallback: if not JSON, try to synthesize minimal data from flat fields
		data = map[string]any{
			"type":    sc.Provider,
			"name":    sc.Label,
			"id":      sc.ID,
			"api_key": plaintext, // last resort for very old flat storage
		}
	}

	// Ensure type/provider present
	if _, ok := data["type"]; !ok && sc.Provider != "" {
		data["type"] = sc.Provider
	}
	if _, ok := data["name"]; !ok && sc.Label != "" {
		data["name"] = sc.Label
	}
	if _, ok := data["id"]; !ok && sc.ID != "" {
		data["id"] = sc.ID
	}

	cred, err := f.FromMap(data)
	if err != nil {
		return nil, fmt.Errorf("credential: from data: %w", err)
	}
	return cred, nil
}

// ToServiceCredential is a thin wrapper that uses DefaultFactory.
func ToServiceCredential(c Credential, cipher *service.Cipher) (*service.Credential, error) {
	return EncryptToService(c, cipher)
}

// FromServiceCredential is a thin wrapper.
func FromServiceCredential(sc *service.Credential, cipher *service.Cipher) (Credential, error) {
	return DecryptFromService(sc, cipher, DefaultFactory)
}
