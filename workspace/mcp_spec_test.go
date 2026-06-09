package workspace

import (
	"encoding/json"
	"testing"
)

func TestParseMCPRegisterBody_FlatSchema(t *testing.T) {
	body := []byte(`{"name":"demo","transport":"stdio","command":"echo","args":["hi"]}`)
	req, spec, err := ParseMCPRegisterBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if req.Name != "demo" || req.Command != "echo" {
		t.Fatalf("unexpected req: %#v", req)
	}
	if spec.Name != "demo" || !spec.IsStateful || spec.MCPConfig.Type != "stdio_mcp" {
		t.Fatalf("unexpected spec: %#v", spec)
	}
}

func TestParseMCPRegisterBody_PyV2Schema(t *testing.T) {
	body := []byte(`{
		"name":"weather",
		"is_stateful":true,
		"mcp_config":{"type":"http_mcp","url":"https://example.com/mcp","headers":{"X":"1"}}
	}`)
	req, spec, err := ParseMCPRegisterBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if req.Transport != "http" || req.Endpoint != "https://example.com/mcp" {
		t.Fatalf("unexpected req: %#v", req)
	}
	if spec.MCPConfig.Type != "http_mcp" || spec.MCPConfig.URL != "https://example.com/mcp" {
		t.Fatalf("unexpected spec: %#v", spec)
	}
}

func TestSpecRoundTrip(t *testing.T) {
	orig := MCPServerSpec{
		Name:       "fs",
		IsStateful: true,
		MCPConfig: MCPConfigSpec{
			Type:    "stdio_mcp",
			Command: "npx",
			Args:    []string{"-y", "pkg"},
			Env:     map[string]string{"A": "B"},
		},
	}
	req, err := RegisterRequestFromSpec(orig)
	if err != nil {
		t.Fatal(err)
	}
	got := SpecFromRegisterRequest(req)
	dataOrig, _ := json.Marshal(orig)
	dataGot, _ := json.Marshal(got)
	if string(dataOrig) != string(dataGot) {
		t.Fatalf("round trip mismatch:\n got %s\nwant %s", dataGot, dataOrig)
	}
}
