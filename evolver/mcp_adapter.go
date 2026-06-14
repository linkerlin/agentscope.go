package evolver

import (
	"context"
	"encoding/json"
	"fmt"
)

// MCPClient is the minimal interface needed to bridge an MCP client into an Evolver.
// toolkit/mcp.SDKClient satisfies this interface.
type MCPClient interface {
	CallTool(ctx context.Context, name string, args map[string]any) (any, error)
}

// NewMCPEvolverFromClient creates an MCPEvolver from any MCPClient implementation.
// This bridges toolkit/mcp.SDKClient (or gateway MCP gateway) to the Evolver interface.
//
// Usage:
//
//	client, _ := mcp.NewSDKClient(stdioTransport)
//	evolver := evolver.NewMCPEvolverFromClient(client)
//	gene, _ := evolver.ListGenes(ctx, "repair")
func NewMCPEvolverFromClient(client MCPClient) *MCPEvolver {
	return NewMCPEvolver(func(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
		raw, err := client.CallTool(ctx, toolName, args)
		if err != nil {
			return nil, fmt.Errorf("mcp call %s: %w", toolName, err)
		}
		return parseMCPResult(raw)
	})
}

// parseMCPResult converts the raw CallTool result into a map[string]any.
// MCP tools typically return text content that may be JSON, or a structured map.
func parseMCPResult(raw any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}

	switch v := raw.(type) {
	case map[string]any:
		return v, nil
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return map[string]any{"result": v}, nil
		}
		return m, nil
	case []byte:
		var m map[string]any
		if err := json.Unmarshal(v, &m); err != nil {
			return map[string]any{"result": string(v)}, nil
		}
		return m, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return map[string]any{"result": fmt.Sprintf("%v", v)}, nil
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return map[string]any{"result": string(data)}, nil
		}
		return m, nil
	}
}
