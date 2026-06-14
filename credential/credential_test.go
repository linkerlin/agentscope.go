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

func TestAllProvidersRoundtrip(t *testing.T) {
	f := NewFactory()

	tests := []struct {
		name    string
		typ     string
		data    map[string]any
		wantErr bool
	}{
		{"dashscope", "dashscope", map[string]any{"type": "dashscope", "name": "ds", "api_key": "sk-ds"}, false},
		{"deepseek", "deepseek", map[string]any{"type": "deepseek", "name": "ds", "api_key": "sk-ds"}, false},
		{"moonshot", "moonshot", map[string]any{"type": "moonshot", "name": "ms", "api_key": "sk-ms"}, false},
		{"xai", "xai", map[string]any{"type": "xai", "name": "xai", "api_key": "sk-xai"}, false},
		{"ollama", "ollama", map[string]any{"type": "ollama", "name": "ol", "base_url": "http://localhost:11434"}, false},
		{"ollama_default", "ollama", map[string]any{"type": "ollama", "name": "ol"}, false},
		{"openai_response", "openai_response", map[string]any{"type": "openai_response", "name": "or", "api_key": "sk-or"}, false},
		{"vllm", "vllm", map[string]any{"type": "vllm", "name": "v", "base_url": "http://gpu:8000"}, false},
		{"dashscope_missing_key", "dashscope", map[string]any{"type": "dashscope", "name": "ds"}, true},
		{"unknown_type", "unknown", map[string]any{"type": "unknown"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := f.FromMap(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(c.Type()) != tt.typ {
				t.Errorf("type mismatch: got %s, want %s", c.Type(), tt.typ)
			}
		})
	}
}

func TestAllSchemasRegistered(t *testing.T) {
	f := NewFactory()
	schemas := f.ListSchemas()
	if len(schemas) != 10 {
		t.Fatalf("expected 10 schemas, got %d", len(schemas))
	}
}

func TestOllamaDefaults(t *testing.T) {
	c := NewOllama("local")
	if c.BaseURL != "http://localhost:11434" {
		t.Errorf("expected default URL, got %s", c.BaseURL)
	}
}

func TestVLLMConstructor(t *testing.T) {
	c := NewVLLM("gpu-node", "http://10.0.0.1:8000")
	if c.BaseURL != "http://10.0.0.1:8000" {
		t.Errorf("base URL mismatch")
	}
	if c.Provider() != "vllm" {
		t.Errorf("provider mismatch")
	}
}
