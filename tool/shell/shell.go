package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// ApprovalCallback is invoked when a command is not in the whitelist.
type ApprovalCallback func(command string) bool

// ShellCommandTool executes shell commands with security validation.
type ShellCommandTool struct {
	AllowedCommands  map[string]struct{}
	ApprovalCallback ApprovalCallback
	Validator        CommandValidator
	BaseDir          string
	DefaultTimeout   time.Duration
}

// NewShellCommandTool creates a ShellCommandTool.
func NewShellCommandTool(baseDir string, allowed []string, callback ApprovalCallback) *ShellCommandTool {
	allowedMap := make(map[string]struct{})
	for _, c := range allowed {
		allowedMap[c] = struct{}{}
	}
	return &ShellCommandTool{
		AllowedCommands:  allowedMap,
		ApprovalCallback: callback,
		Validator:        defaultValidator(),
		BaseDir:          baseDir,
		DefaultTimeout:   300 * time.Second,
	}
}

// Name returns the tool name.
func (s *ShellCommandTool) Name() string { return "execute_shell_command" }

// Description returns the tool description.
func (s *ShellCommandTool) Description() string {
	var parts []string
	parts = append(parts, "Execute a shell command with security validation and return the result.")
	if s.BaseDir != "" {
		parts = append(parts, fmt.Sprintf(" WORKING DIRECTORY: %s", s.BaseDir))
	}
	if len(s.AllowedCommands) > 0 {
		list := make([]string, 0, len(s.AllowedCommands))
		for c := range s.AllowedCommands {
			list = append(list, c)
		}
		parts = append(parts, fmt.Sprintf(" ALLOWED COMMANDS: [%s]", strings.Join(list, ", ")))
	} else {
		parts = append(parts, " No whitelist configured - all commands require approval.")
	}
	parts = append(parts, " Returns output in format: <returncode>code</returncode><stdout>output</stdout><stderr>error</stderr>.")
	return strings.Join(parts, "")
}

// Spec returns the JSON schema.
func (s *ShellCommandTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        s.Name(),
		Description: s.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Maximum time in seconds (default: 300)",
				},
				"working_dir": map[string]any{
					"type":        "string",
					"description": "Optional working directory for the command",
				},
			},
			"required": []string{"command"},
		},
	}
}

// Execute runs the shell command.
func (s *ShellCommandTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	command, _ := input["command"].(string)
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := s.DefaultTimeout
	if t, ok := input["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	} else if t, ok := input["timeout"].(int); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	} else if t, ok := input["timeout"].(string); ok && t != "" {
		if v, err := strconv.Atoi(t); err == nil && v > 0 {
			timeout = time.Duration(v) * time.Second
		}
	}

	workingDir := s.BaseDir
	if wd, ok := input["working_dir"].(string); ok && wd != "" {
		workingDir = wd
	}

	result := s.Validator.Validate(command, s.AllowedCommands)
	if !result.Allowed {
		approved := false
		if s.ApprovalCallback != nil {
			approved = s.ApprovalCallback(command)
		}
		if !approved {
			var errMsg string
			if s.ApprovalCallback == nil {
				errMsg = fmt.Sprintf("SecurityError: %s and no approval callback is configured.", result.Reason)
			} else {
				errMsg = fmt.Sprintf("SecurityError: Command execution was rejected. Reason: %s", result.Reason)
			}
			return tool.NewTextResponse(fmt.Sprintf("<returncode>-1</returncode><stdout></stdout><stderr>%s</stderr>", errMsg)), nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	if workingDir != "" {
		wd, err := filepath.Abs(workingDir)
		if err == nil {
			cmd.Dir = wd
		}
	}

	cmd.Env = os.Environ()
	stdout, err := cmd.Output()
	var stderr []byte
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		} else if ctx.Err() == context.DeadlineExceeded {
			return tool.NewTextResponse(fmt.Sprintf("<returncode>-1</returncode><stdout>%s</stdout><stderr>TimeoutError: command exceeded timeout of %.0f seconds</stderr>", string(stdout), timeout.Seconds())), nil
		} else {
			stderr = []byte(err.Error())
		}
	}

	returnCode := 0
	if cmd.ProcessState != nil {
		returnCode = cmd.ProcessState.ExitCode()
	}
	if err != nil && returnCode == 0 {
		returnCode = -1
	}

	output := fmt.Sprintf("<returncode>%d</returncode><stdout>%s</stdout><stderr>%s</stderr>", returnCode, string(stdout), string(stderr))
	return tool.NewTextResponse(output), nil
}
