package service

import (
	"encoding/base64"
	"testing"
)

func TestCipher_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "my-secret-api-key-12345"
	encrypted, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("encrypted value should differ from plaintext")
	}

	decrypted, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestCipher_DifferentNonces(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, _ := NewCipher(key)

	enc1, _ := c.Encrypt("same")
	enc2, _ := c.Encrypt("same")
	if enc1 == enc2 {
		t.Fatal("same plaintext should produce different ciphertexts due to random nonce")
	}
}

func TestCipher_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1

	c1, _ := NewCipher(key1)
	enc, _ := c1.Encrypt("secret")

	c2, _ := NewCipher(key2)
	_, err := c2.Decrypt(enc)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestCipher_InvalidKeyLength(t *testing.T) {
	_, err := NewCipher([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestCipher_CorruptedData(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewCipher(key)
	enc, _ := c.Encrypt("secret")

	// Corrupt the base64 string
	corrupted := enc[:len(enc)-4] + "XXXX"
	_, err := c.Decrypt(corrupted)
	if err == nil {
		t.Fatal("expected error for corrupted data")
	}
}

func TestCipherFromEnv_Missing(t *testing.T) {
	// Ensure env is not set
	t.Setenv("AGENTSCOPE_ENCRYPTION_KEY", "")
	_, err := NewCipherFromEnv()
	if err == nil {
		t.Fatal("expected error when env is missing")
	}
}

func TestCipherFromEnv_InvalidBase64(t *testing.T) {
	t.Setenv("AGENTSCOPE_ENCRYPTION_KEY", "not-valid-base64!!!")
	_, err := NewCipherFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestCipherFromEnv_WrongLength(t *testing.T) {
	// 16 bytes base64 (too short for AES-256)
	t.Setenv("AGENTSCOPE_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	_, err := NewCipherFromEnv()
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestCipherFromEnv_Success(t *testing.T) {
	t.Setenv("AGENTSCOPE_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	c, err := NewCipherFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enc, _ := c.Encrypt("hello")
	dec, _ := c.Decrypt(enc)
	if dec != "hello" {
		t.Fatalf("round-trip failed: %s", dec)
	}
}

func TestCipher_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewCipher(key)
	enc, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("encrypt empty failed: %v", err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt empty failed: %v", err)
	}
	if dec != "" {
		t.Fatalf("expected empty, got %q", dec)
	}
}

func TestCipher_LongPlaintext(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewCipher(key)
	plaintext := make([]byte, 10000)
	for i := range plaintext {
		plaintext[i] = byte('a' + i%26)
	}
	s := string(plaintext)
	enc, err := c.Encrypt(s)
	if err != nil {
		t.Fatalf("encrypt long failed: %v", err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt long failed: %v", err)
	}
	if dec != s {
		t.Fatal("long plaintext round-trip mismatch")
	}
}

func TestCipher_SpecialCharacters(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewCipher(key)
	plaintext := "Hello \n\t<>\"'& 你好 世界 🌍 🔑 \x00\x01\x02"
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt special chars failed: %v", err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt special chars failed: %v", err)
	}
	if dec != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, dec)
	}
}

func TestCipher_DecryptInvalidBase64(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewCipher(key)
	_, err := c.Decrypt("!!!not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestCipher_DecryptTooShort(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewCipher(key)
	// 4 bytes base64 = 3 raw bytes, shorter than GCM nonce (12)
	_, err := c.Decrypt(base64.StdEncoding.EncodeToString([]byte{1, 2, 3}))
	if err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}
