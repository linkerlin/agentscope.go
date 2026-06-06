package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	"github.com/linkerlin/agentscope.go/tool/shell"
	"github.com/linkerlin/agentscope.go/workspace"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY")
		return
	}

	// ---- 1. Model ----
	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// ---- 2. Workspace (isolated per tenant in production) ----
	wsDir := "./workspace_data"
	_ = os.MkdirAll(wsDir, 0755)
	ws := workspace.NewLocalWorkspace("default", wsDir)

	// ---- 3. Permission engine ----
	// Explore mode: read-only auto-allow, write operations ask.
	permEngine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Target: "tool_name", Pattern: "read_file", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "list_directory", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "shell_command", Decision: permission.DecisionAsk},
	})

	// ---- 4. Tools bound to workspace ----
	tools := []tool.Tool{
		file.NewReadFileTool(wsDir),
		file.NewListDirectoryTool(wsDir),
		file.NewWriteFileTool(wsDir),
		shell.NewShellCommandTool(wsDir, []string{"ls", "cat", "pwd", "echo"}, nil),
	}

	// ---- 5. Agent ----
	agent, err := react.Builder().
		Name("MultiTenantAgent").
		SysPrompt("You are a helpful assistant with access to a local workspace. You can read files, list directories, write files, and run safe shell commands.").
		Model(chatModel).
		Workspace(ws).
		PermissionEngine(permEngine).
		Tools(tools...).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// ---- 6. Service layer (multi-tenant) ----
	storage := service.NewMemoryStorage()
	cipher, _ := service.NewCipherFromEnv() // optional: encrypts credentials if key is set

	// ---- 7. Gateway ----
	srv := gateway.NewServer(agent).
		WithStorage(storage).
		WithCipher(cipher)

	fmt.Println("Multi-tenant workspace example listening on http://localhost:8080")
	fmt.Println("Endpoints:")
	fmt.Println("  POST /api/v1/auth/register   -> register tenant")
	fmt.Println("  POST /api/v1/auth/login      -> login tenant")
	fmt.Println("  POST /api/v1/credentials     -> store API key (encrypted if AGENTSCOPE_ENCRYPTION_KEY is set)")
	fmt.Println("  POST /v2/chat/stream         -> SSE stream with session persistence")
	fmt.Println("  POST /v2/resume              -> resume suspended session")
	if err := http.ListenAndServe(":8080", srv); err != nil {
		log.Fatal(err)
	}
}
