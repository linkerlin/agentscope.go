package shell

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

const (
	// psMaxOutputLen caps stdout+stderr to avoid flooding the context.
	psMaxOutputLen = 30000
	// psDefaultTimeout is the default command timeout.
	psDefaultTimeout = 120 * time.Second
	// psMaxTimeout is the absolute maximum timeout.
	psMaxTimeout = 600 * time.Second
)

var (
	psBinary     string
	psBinaryOnce sync.Once
)

// resolvePowerShell finds pwsh (preferred) or powershell.exe.
// Returns "" if neither is found.
func resolvePowerShell() string {
	psBinaryOnce.Do(func() {
		if p, err := exec.LookPath("pwsh"); err == nil {
			psBinary = p
			return
		}
		if p, err := exec.LookPath("powershell.exe"); err == nil {
			psBinary = p
		}
	})
	return psBinary
}

// encodePowerShellCommand converts a command to a base64 UTF-16-LE encoded
// string suitable for PowerShell's -EncodedCommand flag.
func encodePowerShellCommand(command string) string {
	script := "$ProgressPreference='SilentlyContinue';" +
		"[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;" +
		command
	codes := utf16.Encode([]rune(script))
	buf := make([]byte, len(codes)*2)
	for i, c := range codes {
		buf[i*2] = byte(c)
		buf[i*2+1] = byte(c >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// Backend executes OS commands. LocalBackend runs on the host; tests and
// sandboxes inject fakes or remote runners.
type Backend interface {
	// Run executes name with args in dir and returns stdout, stderr and the
	// process exit code. A non-nil error reports launch/wait failures; the
	// exit code is still meaningful when err is an *exec.ExitError.
	Run(ctx context.Context, name string, args []string, dir string) (stdout, stderr []byte, exitCode int, err error)
}

// LocalBackend executes commands on the host via os/exec.
type LocalBackend struct{}

// Run implements Backend.
func (LocalBackend) Run(ctx context.Context, name string, args []string, dir string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	stdout, err := cmd.Output()
	var stderr []byte
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return stdout, stderr, exitCode, err
}

// PowerShellTool executes PowerShell commands.
// All commands require user confirmation — no command is classified as safe.
//
// Aligned with Python agentscope's PowerShell tool (#798bc181).
type PowerShellTool struct {
	BaseDir        string
	DefaultTimeout time.Duration
	// Backend executes the encoded command (default LocalBackend).
	Backend Backend
}

// NewPowerShellTool creates a PowerShellTool with defaults.
func NewPowerShellTool() *PowerShellTool {
	return &PowerShellTool{
		DefaultTimeout: psDefaultTimeout,
	}
}

// WithBaseDir sets the working directory (builder-style).
func (t *PowerShellTool) WithBaseDir(dir string) *PowerShellTool {
	t.BaseDir = dir
	return t
}

// WithTimeout sets the default timeout (builder-style, capped at psMaxTimeout).
func (t *PowerShellTool) WithTimeout(d time.Duration) *PowerShellTool {
	if d > psMaxTimeout {
		d = psMaxTimeout
	}
	t.DefaultTimeout = d
	return t
}

// WithBackend sets the execution backend (builder-style). A nil backend
// selects LocalBackend.
func (t *PowerShellTool) WithBackend(b Backend) *PowerShellTool {
	t.Backend = b
	return t
}

func (t *PowerShellTool) backend() Backend {
	if t.Backend != nil {
		return t.Backend
	}
	return LocalBackend{}
}

// Name returns the tool name.
func (t *PowerShellTool) Name() string { return "PowerShell" }

// Description returns the tool description.
func (t *PowerShellTool) Description() string {
	desc := "Execute a PowerShell command. All commands require user confirmation."
	if t.BaseDir != "" {
		desc += fmt.Sprintf(" Working directory: %s.", t.BaseDir)
	}
	return desc
}

// Spec returns the JSON schema.
func (t *PowerShellTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The PowerShell command to execute",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Maximum time in milliseconds (default: 120000, max: 600000)",
				},
			},
			"required": []string{"command"},
		},
	}
}

// Execute runs the PowerShell command.
func (t *PowerShellTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	command, _ := input["command"].(string)
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := t.DefaultTimeout
	if ms, ok := input["timeout"].(float64); ok && ms > 0 {
		timeout = time.Duration(ms) * time.Millisecond
	}
	if timeout > psMaxTimeout {
		timeout = psMaxTimeout
	}
	if timeout <= 0 {
		timeout = psDefaultTimeout
	}

	binary := "powershell"
	if t.Backend == nil {
		binary = resolvePowerShell()
		if binary == "" {
			if runtime.GOOS != "windows" {
				return tool.NewTextResponse("<returncode>-1</returncode><stdout></stdout><stderr>PowerShell is not available on this platform.</stderr>"), nil
			}
			return tool.NewTextResponse("<returncode>-1</returncode><stdout></stdout><stderr>PowerShell not found. Install pwsh or powershell.exe.</stderr>"), nil
		}
	}

	encoded := encodePowerShellCommand(command)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout, stderr, returnCode, err := t.backend().Run(ctx, binary,
		[]string{"-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded},
		t.BaseDir)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return tool.NewTextResponse(fmt.Sprintf(
				"<returncode>-1</returncode><stdout>%s</stdout><stderr>TimeoutError: command exceeded timeout of %.0f seconds</stderr>",
				truncOutput(string(stdout)), timeout.Seconds())), nil
		}
		if len(stderr) == 0 {
			stderr = []byte(err.Error())
		}
		if returnCode == 0 {
			returnCode = -1
		}
	}

	output := fmt.Sprintf("<returncode>%d</returncode><stdout>%s</stdout><stderr>%s</stderr>",
		returnCode, truncOutput(string(stdout)), truncOutput(string(stderr)))
	return tool.NewTextResponse(output), nil
}

// IsReadOnly returns false: PowerShell can execute arbitrary commands.
//
// Permission semantics stay always-ask: the tool deliberately implements no
// suggestion generator, so no rule can auto-approve it (only an explicit
// allow rule or bypass mode overrides, PyV2 #798bc181 parity).
func (t *PowerShellTool) IsReadOnly() bool { return false }

// truncOutput caps output to psMaxOutputLen with a truncation marker.
func truncOutput(s string) string {
	if len(s) <= psMaxOutputLen {
		return s
	}
	return s[:psMaxOutputLen] + "\n... [output truncated]"
}
