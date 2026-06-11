package workspace

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCPServerSpec mirrors PyV2 MCPClient.model_dump(mode="json").
// GET /mcps returns this so the host can rebuild GatewayMCPClient losslessly.
type MCPServerSpec struct {
	Name             string        `json:"name"`
	IsStateful       bool          `json:"is_stateful"`
	MCPConfig        MCPConfigSpec `json:"mcp_config"`
	EnableTools      []string      `json:"enable_tools,omitempty"`
	DisableTools     []string      `json:"disable_tools,omitempty"`
	ExecutionTimeout *float64      `json:"execution_timeout,omitempty"`
}

// MCPConfigSpec is the stdio_mcp / http_mcp discriminator union from PyV2.
type MCPConfigSpec struct {
	Type string `json:"type"` // stdio_mcp | http_mcp

	// stdio_mcp
	Command              string            `json:"command,omitempty"`
	Args                 []string          `json:"args,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	Cwd                  string            `json:"cwd,omitempty"`
	EncodingErrorHandler string            `json:"encoding_error_handler,omitempty"`

	// http_mcp
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout *float64          `json:"timeout,omitempty"`
}

// SpecFromRegisterRequest converts the flat gateway POST body to PyV2 spec.
func SpecFromRegisterRequest(req MCPRegisterRequest) MCPServerSpec {
	spec := MCPServerSpec{
		Name:       req.Name,
		IsStateful: true,
	}
	switch strings.ToLower(req.Transport) {
	case "sse", "http":
		spec.MCPConfig = MCPConfigSpec{
			Type:    "http_mcp",
			URL:     req.Endpoint,
			Headers: req.Headers,
		}
	default:
		spec.MCPConfig = MCPConfigSpec{
			Type:    "stdio_mcp",
			Command: req.Command,
			Args:    req.Args,
			Env:     req.Env,
		}
	}
	return spec
}

// RegisterRequestFromSpec converts PyV2 MCPClient spec to flat registration request.
func RegisterRequestFromSpec(spec MCPServerSpec) (MCPRegisterRequest, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return MCPRegisterRequest{}, fmt.Errorf("mcp gateway: name required")
	}
	switch spec.MCPConfig.Type {
	case "stdio_mcp":
		return MCPRegisterRequest{
			Name:      spec.Name,
			Transport: "stdio",
			Command:   spec.MCPConfig.Command,
			Args:      spec.MCPConfig.Args,
			Env:       spec.MCPConfig.Env,
		}, nil
	case "http_mcp":
		return MCPRegisterRequest{
			Name:      spec.Name,
			Transport: "http",
			Endpoint:  spec.MCPConfig.URL,
			Headers:   spec.MCPConfig.Headers,
		}, nil
	case "":
		return MCPRegisterRequest{}, fmt.Errorf("mcp gateway: mcp_config.type required")
	default:
		return MCPRegisterRequest{}, fmt.Errorf("mcp gateway: unknown mcp_config type %q", spec.MCPConfig.Type)
	}
}

// ParseMCPRegisterBody accepts either PyV2 MCPClient spec or the flat POST schema.
func ParseMCPRegisterBody(data []byte) (MCPRegisterRequest, MCPServerSpec, error) {
	var probe struct {
		MCPConfig json.RawMessage `json:"mcp_config"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return MCPRegisterRequest{}, MCPServerSpec{}, err
	}
	if len(probe.MCPConfig) > 0 && string(probe.MCPConfig) != "null" {
		var spec MCPServerSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			return MCPRegisterRequest{}, MCPServerSpec{}, err
		}
		req, err := RegisterRequestFromSpec(spec)
		if err != nil {
			return MCPRegisterRequest{}, MCPServerSpec{}, err
		}
		if !spec.IsStateful {
			spec.IsStateful = true
		}
		return req, spec, nil
	}
	var req MCPRegisterRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return MCPRegisterRequest{}, MCPServerSpec{}, err
	}
	return req, SpecFromRegisterRequest(req), nil
}
