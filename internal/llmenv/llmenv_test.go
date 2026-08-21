package llmenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrefersEnvOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "OPENAI_API_KEY=file-key\nOPENAI_BASE_URL=https://file.example/v1\nOPENAI_MODEL=file-model\n")
	t.Chdir(dir)

	clearOpenAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "env-key")
	c := Load()
	if c.APIKey != "env-key" {
		t.Fatalf("env should win over .env, got %q", c.APIKey)
	}
	if c.BaseURL != "https://file.example/v1" || c.Model != "file-model" {
		t.Fatalf("missing vars should fall back to .env, got %+v", c)
	}
}

func TestLoadDotEnvSyntax(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "# comment\n\nexport OPENAI_API_KEY=\"quoted\"\nOPENAI_BASE_URL='single'\nbroken-line-no-equals\n")
	t.Chdir(dir)
	clearOpenAIEnv(t)

	c := Load()
	if c.APIKey != "quoted" || c.BaseURL != "single" {
		t.Fatalf("unexpected config: %+v", c)
	}
	if c.Model != DefaultModel {
		t.Fatalf("model should default to %q, got %q", DefaultModel, c.Model)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // 无 .env，三个变量均空
	clearOpenAIEnv(t)

	c := Load()
	if c.APIKey != "" || c.BaseURL != "" || c.Model != DefaultModel {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func clearOpenAIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL"} {
		t.Setenv(k, "")
	}
}

func writeEnv(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
