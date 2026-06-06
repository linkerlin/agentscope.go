package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

// Cipher provides AES-GCM encryption/decryption for sensitive data such as
// API keys. Each encryption generates a random nonce; the ciphertext format is
// base64(nonce || ciphertext) so that decryption can recover the nonce.
type Cipher struct {
	gcm cipher.AEAD
}

// NewCipher creates a Cipher from a raw key. For AES-256-GCM the key must be
// exactly 32 bytes.
func NewCipher(key []byte) (*Cipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: failed to create AES block: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher: failed to create GCM: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// NewCipherFromEnv creates a Cipher from the AGENTSCOPE_ENCRYPTION_KEY
// environment variable. The value must be base64-encoded and decode to exactly
// 32 bytes (AES-256).
func NewCipherFromEnv() (*Cipher, error) {
	env := os.Getenv("AGENTSCOPE_ENCRYPTION_KEY")
	if env == "" {
		return nil, errors.New("cipher: AGENTSCOPE_ENCRYPTION_KEY is not set")
	}
	key, err := base64.StdEncoding.DecodeString(env)
	if err != nil {
		return nil, fmt.Errorf("cipher: AGENTSCOPE_ENCRYPTION_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("cipher: AGENTSCOPE_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
	}
	return NewCipher(key)
}

// Encrypt encrypts plaintext and returns base64(nonce || ciphertext).
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("cipher: failed to generate nonce: %w", err)
	}
	ciphertext := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64(nonce || ciphertext) and returns plaintext.
func (c *Cipher) Decrypt(ciphertextB64 string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("cipher: failed to decode base64: %w", err)
	}
	nonceSize := c.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("cipher: ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("cipher: decryption failed (wrong key or corrupted data): %w", err)
	}
	return string(plaintext), nil
}
