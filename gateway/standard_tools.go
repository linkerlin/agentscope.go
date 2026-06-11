package gateway

import (
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	jsont "github.com/linkerlin/agentscope.go/tool/json"
	scheduletask "github.com/linkerlin/agentscope.go/tool/task"
	"github.com/linkerlin/agentscope.go/tool/web"

	scheduletool "github.com/linkerlin/agentscope.go/tool/schedule"
)

// StandardToolsOptions controls what is included in the standard tool set.
type StandardToolsOptions struct {
	WorkspaceDir    string
	ReadOnly        bool
	IncludeWeb      bool
	IncludeJSON     bool
	IncludeTask     bool
	IncludeSchedule bool
	ScheduleMgr     scheduletool.Manager // required if IncludeSchedule
	TaskStore       scheduletask.Store   // required if IncludeTask
	Extra           []tool.Tool
}

// StandardTools returns a ready-to-use set of common tools for ReAct agents
// in a multi-tenant / workspace-backed environment.
//
// It is the Go equivalent of easily assembling the Python builtin tools
// (Bash/Read/Write/Edit/Glob/Grep + Task* + Schedule* + Web + JSON + ...).
//
// Example:
//
//	tools := gateway.StandardTools(gateway.StandardToolsOptions{
//	    WorkspaceDir: "/tmp/ws",
//	    IncludeSchedule: true,
//	    ScheduleMgr: myScheduleMgr,
//	})
//	agent, _ := react.Builder().Tools(tools...).Build()
func StandardTools(opts StandardToolsOptions) []tool.Tool {
	dir := opts.WorkspaceDir
	if dir == "" {
		dir = "."
	}

	// Auto-create simple in-memory TaskStore when IncludeTask but no store provided.
	// This makes StandardTools(self-contained) for common cases.
	if opts.IncludeTask && opts.TaskStore == nil {
		opts.TaskStore = state.NewTaskStore()
	}

	tools := file.RegisterAll(dir, opts.ReadOnly)

	if opts.IncludeTask && opts.TaskStore != nil {
		tools = append(tools, scheduletask.RegisterTools(opts.TaskStore)...)
	}

	if opts.IncludeSchedule && opts.ScheduleMgr != nil {
		tools = append(tools, scheduletool.RegisterTools(opts.ScheduleMgr)...)
	}

	if opts.IncludeWeb {
		tools = append(tools, web.NewFetchTool(0)) // default timeout
	}

	if opts.IncludeJSON {
		tools = append(tools, jsont.NewParseTool(), jsont.NewQueryTool())
	}

	tools = append(tools, opts.Extra...)
	return tools
}
