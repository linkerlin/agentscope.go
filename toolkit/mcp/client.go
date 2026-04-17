package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// SDKClient 是基于 mark3labs/mcp-go 的 Client 实现
type SDKClient struct {
	inner       *mcpclient.Client
	initReq     mcp.InitializeRequest
	initialized bool
	transport   string
}

// NewSDKClient 包装一个已创建的 mcp-go client（尚未 Initialize）
func NewSDKClient(inner *mcpclient.Client) *SDKClient {
	return &SDKClient{inner: inner}
}

// Connect 执行 Initialize（幂等：若已初始化则直接返回）
func (c *SDKClient) Connect(ctx context.Context, cfg MCPConfig) error {
	if c.initialized {
		return nil
	}

	// SSE transport 需要显式 Start（stdio 在创建时已自动 start，http 不需要）
	if c.transport == "sse" {
		if err := c.inner.Start(ctx); err != nil {
			return fmt.Errorf("mcp: start failed: %w", err)
		}
	}

	initReq := c.initReq
	if initReq.Params.ProtocolVersion == "" {
		initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	}
	if initReq.Params.ClientInfo.Name == "" {
		initReq.Params.ClientInfo = mcp.Implementation{
			Name:    "agentscope.go",
			Version: "1.0.0",
		}
	}

	_, err := c.inner.Initialize(ctx, initReq)
	if err != nil {
		return fmt.Errorf("mcp: initialize failed: %w", err)
	}
	c.initialized = true
	return nil
}

func (c *SDKClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	if !c.initialized {
		return nil, fmt.Errorf("mcp: client not initialized")
	}
	res, err := c.inner.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	var infos []ToolInfo
	for _, t := range res.Tools {
		infos = append(infos, convertTool(t))
	}
	return infos, nil
}

func (c *SDKClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if !c.initialized {
		return nil, fmt.Errorf("mcp: client not initialized")
	}
	res, err := c.inner.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, err
	}
	return formatCallToolResult(res), nil
}

func (c *SDKClient) Close() error {
	return c.inner.Close()
}

func convertTool(t mcp.Tool) ToolInfo {
	schemaMap := map[string]any{
		"type":       t.InputSchema.Type,
		"properties": t.InputSchema.Properties,
	}
	if len(t.InputSchema.Required) > 0 {
		schemaMap["required"] = t.InputSchema.Required
	}
	if t.InputSchema.AdditionalProperties != nil {
		schemaMap["additionalProperties"] = t.InputSchema.AdditionalProperties
	}
	if len(t.InputSchema.Defs) > 0 {
		schemaMap["$defs"] = t.InputSchema.Defs
	}
	return ToolInfo{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  schemaMap,
	}
}

func formatCallToolResult(res *mcp.CallToolResult) any {
	if res == nil {
		return ""
	}
	var parts []string
	for _, c := range res.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			parts = append(parts, v.Text)
		case *mcp.TextContent:
			parts = append(parts, v.Text)
		default:
			b, _ := json.Marshal(c)
			parts = append(parts, string(b))
		}
	}
	if len(parts) == 0 {
		if res.IsError {
			return "error"
		}
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	var out string
	for _, p := range parts {
		if p != "" {
			out += p + "\n"
		}
	}
	return out
}

// ClientBuilder 用于构建基于 mcp-go 的 Client（对齐 Java McpClientBuilder）
type ClientBuilder struct {
	name        string
	transport   string // stdio | sse | http
	command     string
	args        []string
	env         map[string]string
	baseURL     string
	headers     map[string]string
	queryParams map[string]string
	timeout     time.Duration
	initTimeout time.Duration
	elicitation func(ctx context.Context, req ElicitRequest) (ElicitResult, error)
}

// NewClientBuilder 创建 Builder
func NewClientBuilder(name string) *ClientBuilder {
	return &ClientBuilder{
		name:        name,
		headers:     make(map[string]string),
		queryParams: make(map[string]string),
		timeout:     120 * time.Second,
		initTimeout: 30 * time.Second,
	}
}

// StdioTransport 配置本地子进程传输
func (b *ClientBuilder) StdioTransport(command string, args ...string) *ClientBuilder {
	b.transport = "stdio"
	b.command = command
	b.args = args
	return b
}

// StdioTransportWithEnv 配置本地子进程传输并携带环境变量
func (b *ClientBuilder) StdioTransportWithEnv(command string, env map[string]string, args ...string) *ClientBuilder {
	b.transport = "stdio"
	b.command = command
	b.env = env
	b.args = args
	return b
}

// SSETransport 配置 SSE 传输
func (b *ClientBuilder) SSETransport(baseURL string) *ClientBuilder {
	b.transport = "sse"
	b.baseURL = baseURL
	return b
}

// StreamableHTTPTransport 配置 Streamable HTTP 传输
func (b *ClientBuilder) StreamableHTTPTransport(baseURL string) *ClientBuilder {
	b.transport = "http"
	b.baseURL = baseURL
	return b
}

// Header 添加 HTTP 请求头（仅 SSE/HTTP 生效）
func (b *ClientBuilder) Header(key, value string) *ClientBuilder {
	b.headers[key] = value
	return b
}

// QueryParam 添加 URL 查询参数（仅 SSE/HTTP 生效）
func (b *ClientBuilder) QueryParam(key, value string) *ClientBuilder {
	b.queryParams[key] = value
	return b
}

// Timeout 设置请求超时
func (b *ClientBuilder) Timeout(d time.Duration) *ClientBuilder {
	b.timeout = d
	return b
}

// InitializationTimeout 设置初始化超时
func (b *ClientBuilder) InitializationTimeout(d time.Duration) *ClientBuilder {
	b.initTimeout = d
	return b
}

// Elicitation 注册协商回调
func (b *ClientBuilder) Elicitation(handler func(ctx context.Context, req ElicitRequest) (ElicitResult, error)) *ClientBuilder {
	b.elicitation = handler
	return b
}

// Build 创建并返回 Client（尚未 Initialize，需调用 Connect）
func (b *ClientBuilder) Build() (Client, error) {
	if b.transport == "" {
		return nil, fmt.Errorf("mcp: transport not configured")
	}

	var raw *mcpclient.Client
	var err error

	switch b.transport {
	case "stdio":
		var envList []string
		if len(b.env) > 0 {
			for k, v := range b.env {
				envList = append(envList, k+"="+v)
			}
		}
		raw, err = mcpclient.NewStdioMCPClient(b.command, envList, b.args...)
		if err != nil {
			return nil, err
		}
	case "sse":
		endpoint := b.mergeEndpoint()
		var opts []mcptransport.ClientOption
		if len(b.headers) > 0 {
			opts = append(opts, mcptransport.WithHeaders(b.headers))
		}
		if b.timeout > 0 {
			opts = append(opts, mcptransport.WithEndpointTimeout(b.timeout))
		}
		raw, err = mcpclient.NewSSEMCPClient(endpoint, opts...)
		if err != nil {
			return nil, err
		}
	case "http":
		endpoint := b.mergeEndpoint()
		var opts []mcptransport.StreamableHTTPCOption
		if len(b.headers) > 0 {
			opts = append(opts, mcptransport.WithHTTPHeaders(b.headers))
		}
		if b.timeout > 0 {
			opts = append(opts, mcptransport.WithHTTPTimeout(b.timeout))
		}
		raw, err = mcpclient.NewStreamableHttpClient(endpoint, opts...)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("mcp: unknown transport %s", b.transport)
	}

	c := &SDKClient{inner: raw, transport: b.transport}
	if b.elicitation != nil {
		// SDKClient 本身不直接处理 elicitation；这里仅保留扩展空间
		// mcp-go 的 client 是否支持 elicitation 需要更高层封装
		_ = b.elicitation
	}
	return c, nil
}

func (b *ClientBuilder) mergeEndpoint() string {
	u, err := url.Parse(b.baseURL)
	if err != nil {
		return b.baseURL
	}
	q := u.Query()
	for k, v := range b.queryParams {
		q.Set(k, v)
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String()
}
