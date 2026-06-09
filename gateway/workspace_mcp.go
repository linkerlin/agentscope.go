package gateway

import (
	"encoding/json"
	"fmt"

	"github.com/linkerlin/agentscope.go/workspace"
)

// parseMCPAddRequest accepts PyV2 MCPClient JSON or legacy {name,gateway_url,token}.
func parseMCPAddRequest(data []byte, defaultGatewayURL, defaultGatewayToken string) (MCPRegistration, error) {
	var probe struct {
		MCPConfig json.RawMessage `json:"mcp_config"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return MCPRegistration{}, err
	}

	var reg MCPRegistration
	if len(probe.MCPConfig) > 0 && string(probe.MCPConfig) != "null" {
		var combined struct {
			workspace.MCPServerSpec
			GatewayURL string `json:"gateway_url"`
			Token      string `json:"token"`
		}
		if err := json.Unmarshal(data, &combined); err != nil {
			return MCPRegistration{}, err
		}
		reg = MCPRegistration{
			Name:       combined.Name,
			GatewayURL: combined.GatewayURL,
			Token:      combined.Token,
			Spec:       combined.MCPServerSpec,
		}
	} else {
		if err := json.Unmarshal(data, &reg); err != nil {
			return MCPRegistration{}, err
		}
	}

	if reg.GatewayURL == "" {
		reg.GatewayURL = defaultGatewayURL
	}
	if reg.Token == "" {
		reg.Token = defaultGatewayToken
	}
	if reg.Name == "" {
		return MCPRegistration{}, fmt.Errorf("name is required")
	}
	if reg.GatewayURL == "" {
		return MCPRegistration{}, fmt.Errorf("gateway_url is required")
	}
	if reg.Spec.Name == "" {
		reg.Spec.Name = reg.Name
	}
	return reg, nil
}
