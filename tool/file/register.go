package file

import (
	"github.com/linkerlin/agentscope.go/tool"
)

// RegisterAll returns all built-in file tools for a given base directory.
// If readOnly is true, only read-only tools are registered
// (ReadFile, Glob, Grep). Otherwise, write tools (Write, Edit) are included.
func RegisterAll(baseDir string, readOnly bool) []tool.Tool {
	tools := []tool.Tool{
		NewReadFileTool(baseDir),
		NewGlobTool(baseDir),
		NewGrepTool(baseDir),
	}
	if !readOnly {
		tools = append(tools,
			NewWriteFileTool(baseDir),
			NewEditFileTool(baseDir),
			NewInsertTextFileTool(baseDir),
		)
	}
	return tools
}
