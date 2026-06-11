package credential

import (
	"testing"

	"github.com/linkerlin/agentscope.go/service"
)

func TestFactoryRegisterAndFromMap(t *testing.T) {
	f := NewFactory()

	data := map[string]any{
		"type":     "openai",
		"name":     "My OpenAI Key",
		"api_key":  "sk-test-123",
		"base_url": "https://api.openai.com/v1",
	}

	c, err := f.FromMap(data)
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	oc, ok := c.(*OpenAICredential)
	if !ok {
		t.Fatalf("expected *OpenAICredential, got %T", c)
	}
	if oc.APIKey != "sk-test-123" {
		t.Errorf("APIKey mismatch: %s", oc.APIKey)
	}
	if oc.Provider() != "openai" {
		t.Errorf("Provider mismatch")
	}
}

func TestListSchemas(t *testing.T) {
	f := NewFactory()
	schemas := f.ListSchemas()
	if len(schemas) == 0 {
		t.Fatal("expected at least one schema")
	}
	foundOpenAI := false
	for _, s := range schemas {
		if title, _ := s["title"].(string); title == "OpenAI API" {
			foundOpenAI = true
			break
		}
	}
	if !foundOpenAI {
		t.Error("OpenAI schema not found in ListSchemas")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	cipher, err := service.NewCipher([]byte("test-secret-32-bytes-long!!!!!!!")) // 32 bytes
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	orig := NewOpenAI("Test Key", "sk-very-secret")

	sc, err := EncryptToService(orig, cipher)
	if err != nil {
		t.Fatalf("EncryptToService: %v", err)
	}
	if sc.Provider != "openai" {
		t.Errorf("provider not set")
	}
	if sc.Encrypted == "" {
		t.Error("encrypted blob empty")
	}

	restored, err := DecryptFromService(sc, cipher, DefaultFactory)
	if err != nil {
		t.Fatalf("DecryptFromService: %v", err)
	}

	oc, ok := restored.(*OpenAICredential)
	if !ok {
		t.Fatalf("restored type %T", restored)
	}
	if oc.APIKey != "sk-very-secret" {
		t.Errorf("api key not restored: %s", oc.APIKey)
	}
	if oc.Name() != "Test Key" {
		t.Errorf("name not restored")
	}
}

func TestAnthropic(t *testing.T) {
	c := NewAnthropic("Claude Key", "sk-ant-xxx")
	if c.Provider() != "anthropic" {
		t.Error("wrong provider")
	}
	data := c.ToData()
	if data["api_key"] != "sk-ant-xxx" {
		t.Error("data missing key")
	}

	f := NewFactory()
	c2, err := f.FromMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Provider() != "anthropic" {
		t.Error("roundtrip provider")
	}
}
