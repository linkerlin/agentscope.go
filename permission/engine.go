// Package permission provides a rule-based permission engine for agent tool execution.
// It supports glob path matching, command substring matching, safety checks for
// dangerous paths and commands, and HITL (Human-in-the-Loop) suspend-resume via
// the ASK decision.
package permission

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// Mode defines the default behavior when no explicit rule matches.
type Mode string

const (
	ModeDefault     Mode = "default"      // all operations require explicit permission
	ModeExplore     Mode = "explore"      // read-only auto-allow
	ModeAcceptEdits Mode = "accept_edits" // accept edit operations
	ModeBypass      Mode = "bypass"       // allow everything
	ModeDontAsk     Mode = "dont_ask"     // silent mode
)

// Decision is the outcome of a permission evaluation for a single action.
type Decision string

const (
	DecisionAllow       Decision = "allow"
	DecisionDeny        Decision = "deny"
	DecisionAsk         Decision = "ask"
	DecisionPassthrough Decision = "passthrough"
)

// Rule is a single permission rule.
type Rule struct {
	Name     string   `json:"name"`
	ToolName string   `json:"tool_name,omitempty"` // optional: exact tool name match
	Target   string   `json:"target"`              // "tool_name" | "file_path" | "command"
	Pattern  string   `json:"pattern"`             // glob for file_path, substring for command/tool_name
	Decision Decision `json:"decision"`            // allow | deny | ask | passthrough
}

// Result holds the result for a single tool call.
type Result struct {
	ToolCallID     string
	ToolName       string
	Decision       Decision
	Message        string
	Reason         string
	SuggestedRules []Rule
}

// Evaluation is an alias for Result for backward compatibility.
type Evaluation = Result

// Engine evaluates permission rules against tool calls.
type Engine struct {
	ctx           *Context
	rules         []Rule
	readOnlyTools map[string]bool
	toolResolver  func(string) tool.Tool
}

// NewEngine creates a permission engine with the given mode and rules.
func NewEngine(mode Mode, rules []Rule) *Engine {
	if mode == "" {
		mode = ModeExplore
	}
	return &Engine{
		ctx:           NewContext(mode),
		rules:         rules,
		readOnlyTools: defaultReadOnlyTools(),
	}
}

// NewEngineWithContext creates a permission engine with a full context.
func NewEngineWithContext(ctx *Context, rules []Rule) *Engine {
	if ctx == nil {
		ctx = NewContext(ModeExplore)
	}
	return &Engine{
		ctx:           ctx,
		rules:         rules,
		readOnlyTools: defaultReadOnlyTools(),
	}
}

// WithReadOnlyTools sets which tool names are considered read-only.
func (e *Engine) WithReadOnlyTools(names ...string) *Engine {
	e.readOnlyTools = make(map[string]bool)
	for _, n := range names {
		e.readOnlyTools[n] = true
	}
	return e
}

func defaultReadOnlyTools() map[string]bool {
	return map[string]bool{
		"read_file":      true,
		"list_directory": true,
		"glob":           true,
		"grep":           true,
		"read":           true,
	}
}

// SetToolResolver wires tool lookup for per-tool permission hooks (PyV2 check_permissions / match_rule).
func (e *Engine) SetToolResolver(resolver func(string) tool.Tool) {
	e.toolResolver = resolver
}

func (e *Engine) resolveTool(name string) tool.Tool {
	if e.toolResolver == nil {
		return nil
	}
	return e.toolResolver(name)
}

// It returns a per-tool-call decision. If any decision is ASK, the caller
// (typically the agent) should suspend and emit a RequireUserConfirmEvent.
func (e *Engine) Evaluate(toolCalls []*message.ToolUseBlock) ([]Result, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}

	grouped := e.groupRules()
	results := make([]Result, len(toolCalls))
	for i, tc := range toolCalls {
		results[i] = e.evaluateOne(tc, grouped)
	}
	return results, nil
}

func (e *Engine) groupRules() map[Decision]map[string][]Rule {
	groups := make(map[Decision]map[string][]Rule)
	for _, d := range []Decision{DecisionDeny, DecisionAsk, DecisionAllow} {
		groups[d] = make(map[string][]Rule)
	}
	for _, r := range e.rules {
		byDecision := groups[r.Decision]
		if byDecision == nil {
			continue
		}
		key := r.ToolName // empty = global
		byDecision[key] = append(byDecision[key], r)
	}
	return groups
}

func (e *Engine) evaluateOne(tc *message.ToolUseBlock, grouped map[Decision]map[string][]Rule) Result {
	filePath := extractString(tc.Input, "file_path", "dir_path", "path")
	command := extractString(tc.Input, "command")
	t := e.resolveTool(tc.Name)
	isReadOnly := e.readOnlyTools[tc.Name]
	if t != nil {
		if ro, ok := t.(tool.ReadOnlyChecker); ok && ro.IsReadOnly() {
			isReadOnly = true
		}
	}

	input := tc.Input
	if input == nil {
		input = map[string]any{}
	}

	// 1. Deny rules (highest priority)
	if r := e.matchRules(grouped[DecisionDeny], tc.Name, filePath, command, input, t); r != nil {
		return Result{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   DecisionDeny,
			Message:    fmt.Sprintf("Permission denied for %s", tc.Name),
			Reason:     fmt.Sprintf("matched deny rule %q", r.Name),
		}
	}

	// 2. Ask rules
	if r := e.matchRules(grouped[DecisionAsk], tc.Name, filePath, command, input, t); r != nil {
		return Result{
			ToolCallID:     tc.ID,
			ToolName:       tc.Name,
			Decision:       DecisionAsk,
			Message:        fmt.Sprintf("Permission required for %s", tc.Name),
			Reason:         fmt.Sprintf("matched ask rule %q", r.Name),
			SuggestedRules: e.generateSuggestions(tc, filePath, command, t),
		}
	}

	// 3. Tool-specific permission checks (PyV2 check_permissions)
	if t != nil {
		if decider, ok := t.(tool.PermissionDecider); ok {
			dec, msg, reason, passthrough := decider.CheckPermissions(input, e.ctx)
			if !passthrough {
				engineDec := fromToolDecision(dec)
				result := Result{
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Decision:   engineDec,
					Message:    msg,
					Reason:     reason,
				}
				if engineDec == DecisionAsk {
					result.SuggestedRules = e.generateSuggestions(tc, filePath, command, t)
				}
				return result
			}
		}
	}

	// 4. Tool-specific safety checks (bypass-immune)
	if safety := e.checkSafety(tc.Name, filePath, command, isReadOnly); safety != nil {
		// Copy tool call ID into safety result.
		safety.ToolCallID = tc.ID
		return *safety
	}

	// 5. Allow rules
	if r := e.matchRules(grouped[DecisionAllow], tc.Name, filePath, command, input, t); r != nil {
		return Result{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   DecisionAllow,
			Message:    fmt.Sprintf("Permission granted for %s", tc.Name),
			Reason:     fmt.Sprintf("matched allow rule %q", r.Name),
		}
	}

	// 6. Mode defaults
	return e.defaultResult(tc, isReadOnly, filePath, command, t)
}

func (e *Engine) matchRules(rules map[string][]Rule, toolName, filePath, command string, input map[string]any, t tool.Tool) *Rule {
	for _, key := range []string{toolName, ""} {
		for i := range rules[key] {
			r := &rules[key][i]
			if e.ruleMatches(r, toolName, filePath, command, input, t) {
				return r
			}
		}
	}
	return nil
}

func (e *Engine) ruleMatches(r *Rule, toolName, filePath, command string, input map[string]any, t tool.Tool) bool {
	if t != nil {
		if matcher, ok := t.(tool.RuleMatcher); ok {
			if r.Pattern == "" {
				return true
			}
			return matcher.MatchRule(r.Pattern, input)
		}
	}
	matched := false
	switch r.Target {
	case "tool_name":
		matched = matchGlob(r.Pattern, toolName)
	case "file_path":
		matched = matchGlob(r.Pattern, filePath)
	case "command":
		matched = matchGlobOrRegex(r.Pattern, command)
	case "regex":
		matched = matchRegex(r.Pattern, toolName+" "+filePath+" "+command)
	default:
		matched = matchGlobOrRegex(r.Pattern, toolName)
	}
	return matched
}

// checkSafety performs bypass-immune safety checks.
func (e *Engine) checkSafety(toolName, filePath, command string, isReadOnly bool) *Result {
	// EXPLORE mode: deny modifications (bypass-immune).
	if e.ctx.Mode == ModeExplore && !isReadOnly {
		return &Result{
			ToolName: toolName,
			Decision: DecisionDeny,
			Message:  fmt.Sprintf("Permission denied for %s (explore mode is read-only)", toolName),
			Reason:   "Explore mode does not allow modifications",
		}
	}

	// Bash-specific safety checks.
	if isBashTool(toolName) && command != "" {
		if IsDangerousCommand(command) {
			return &Result{
				ToolName: toolName,
				Decision: DecisionAsk,
				Message:  fmt.Sprintf("Permission required: dangerous command pattern detected in %q", command),
				Reason:   "Safety check: dangerous command pattern detected",
			}
		}
		if IsReadOnlyCommand(command) {
			return &Result{
				ToolName: toolName,
				Decision: DecisionAllow,
				Message:  "Permission granted for read-only command",
				Reason:   "Read-only command is allowed",
			}
		}
		if IsDangerousRemoval(command) {
			return &Result{
				ToolName: toolName,
				Decision: DecisionAsk,
				Message:  fmt.Sprintf("Dangerous removal operation detected in %q", command),
				Reason:   "Safety check: dangerous removal of critical system path",
			}
		}
	}

	// Write-specific dangerous path check.
	if isWriteFileTool(toolName) && filePath != "" {
		if IsDangerousPath(filePath, e.ctx.DangerousFiles, e.ctx.DangerousDirs) {
			return &Result{
				ToolName: toolName,
				Decision: DecisionAsk,
				Message:  fmt.Sprintf("Permission required: write operation on sensitive file %s", filePath),
				Reason:   "Safety check: dangerous file or directory",
			}
		}
	}

	// ACCEPT_EDITS mode: auto-allow filesystem commands in working dirs.
	if e.ctx.Mode == ModeAcceptEdits {
		if isReadOnly {
			return &Result{
				ToolName: toolName,
				Decision: DecisionAllow,
				Message:  fmt.Sprintf("Permission granted for %s (accept edits mode - read-only tool)", toolName),
				Reason:   "Accept edits mode allows read-only operations",
			}
		}
		if filePath != "" && e.pathInWorkingDirs(filePath) {
			return &Result{
				ToolName: toolName,
				Decision: DecisionAllow,
				Message:  fmt.Sprintf("Permission granted for %s (accept edits mode - in working directory)", toolName),
				Reason:   "File is in working directory and not a dangerous path",
			}
		}
	}

	return nil
}

func (e *Engine) pathInWorkingDirs(filePath string) bool {
	if filePath == "" {
		return false
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		absFile = filePath
	}
	for _, wd := range e.ctx.WorkingDirs {
		absWd, err := filepath.Abs(wd)
		if err != nil {
			absWd = wd
		}
		sep := string(filepath.Separator)
		if strings.HasPrefix(absFile, absWd+sep) || absFile == absWd {
			return true
		}
	}
	return false
}

func (e *Engine) defaultResult(tc *message.ToolUseBlock, isReadOnly bool, filePath, command string, t tool.Tool) Result { //nolint:unparam // isReadOnly reserved for future mode logic
	switch e.ctx.Mode {
	case ModeBypass:
		return Result{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   DecisionAllow,
			Message:    fmt.Sprintf("Permission granted for %s (bypass mode)", tc.Name),
			Reason:     "Bypass mode allows all operations",
		}
	case ModeDontAsk:
		return Result{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   DecisionDeny,
			Message:    fmt.Sprintf("Permission denied for %s (dont_ask mode - user not available)", tc.Name),
			Reason:     "User is not available to answer permission prompts",
		}
	case ModeExplore:
		// Safety check already handled non-read-only.
		return Result{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   DecisionAllow,
			Message:    fmt.Sprintf("Permission granted for %s (explore mode)", tc.Name),
			Reason:     "Explore mode allows read-only operations",
		}
	case ModeAcceptEdits:
		if isEditTool(tc.Name) {
			return Result{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Decision:   DecisionAllow,
				Message:    fmt.Sprintf("Permission granted for %s (accept edits mode)", tc.Name),
				Reason:     "Accept edits mode allows file edits",
			}
		}
		if isWriteTool(tc.Name) {
			return Result{
				ToolCallID:     tc.ID,
				ToolName:       tc.Name,
				Decision:       DecisionAsk,
				Message:        fmt.Sprintf("Permission required for %s", tc.Name),
				Reason:         "Accept edits mode requires explicit permission for non-edit write operations",
				SuggestedRules: e.generateSuggestions(tc, filePath, command, t),
			}
		}
		return Result{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   DecisionAllow,
			Message:    fmt.Sprintf("Permission granted for %s (accept edits mode)", tc.Name),
			Reason:     "Accept edits mode allows read-only operations",
		}
	default: // ModeDefault
		return Result{
			ToolCallID:     tc.ID,
			ToolName:       tc.Name,
			Decision:       DecisionAsk,
			Message:        fmt.Sprintf("Permission required for %s", tc.Name),
			Reason:         "Default mode requires explicit permission for each action",
			SuggestedRules: e.generateSuggestions(tc, filePath, command, t),
		}
	}
}

func (e *Engine) generateSuggestions(tc *message.ToolUseBlock, filePath, command string, t tool.Tool) []Rule {
	if t != nil {
		if gen, ok := t.(tool.SuggestionGenerator); ok {
			suggestions := gen.GenerateSuggestions(tc.Input)
			if len(suggestions) > 0 {
				return fromToolSuggestions(suggestions)
			}
		}
	}
	var suggestions []Rule
	if command != "" {
		parts := strings.Fields(command)
		if len(parts) >= 2 {
			suggestions = append(suggestions, Rule{
				Name:     "suggested-bash-prefix",
				ToolName: tc.Name,
				Target:   "command",
				Pattern:  parts[0] + " " + parts[1] + ":*",
				Decision: DecisionAllow,
			})
		} else if len(parts) == 1 {
			suggestions = append(suggestions, Rule{
				Name:     "suggested-bash-cmd",
				ToolName: tc.Name,
				Target:   "command",
				Pattern:  parts[0],
				Decision: DecisionAllow,
			})
		}
	}
	if filePath != "" {
		dir := filepath.Dir(filePath)
		if dir != "" && dir != "." {
			pattern := dir + string(filepath.Separator) + "*"
			suggestions = append(suggestions, Rule{
				Name:     "suggested-file-dir",
				ToolName: tc.Name,
				Target:   "file_path",
				Pattern:  pattern,
				Decision: DecisionAllow,
			})
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, Rule{
			Name:     "suggested-tool-level",
			ToolName: tc.Name,
			Target:   "tool_name",
			Pattern:  tc.Name,
			Decision: DecisionAllow,
		})
	}
	return suggestions
}

func isBashTool(name string) bool {
	switch name {
	case "execute_shell_command", "shell_command", "bash":
		return true
	}
	return false
}

func isWriteFileTool(name string) bool {
	switch name {
	case "write_text_file", "write_file", "insert_text_file", "edit_file":
		return true
	}
	return false
}

func isWriteTool(name string) bool {
	return isWriteFileTool(name) || isBashTool(name)
}

func isEditTool(name string) bool {
	return isWriteFileTool(name)
}

func extractString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

func fromToolDecision(dec tool.PermissionDecision) Decision {
	switch dec {
	case tool.PermAllow:
		return DecisionAllow
	case tool.PermDeny:
		return DecisionDeny
	case tool.PermAsk:
		return DecisionAsk
	default:
		return DecisionAsk
	}
}

func fromToolSuggestions(in []tool.SuggestedRule) []Rule {
	out := make([]Rule, 0, len(in))
	for _, s := range in {
		out = append(out, Rule{
			Name:     s.Name,
			ToolName: s.ToolName,
			Target:   s.Target,
			Pattern:  s.Pattern,
			Decision: fromToolDecision(s.Decision),
		})
	}
	return out
}

// matchGlob performs a simple glob match. It uses filepath.Match for full
// glob support. A pattern of "*" matches everything.
func matchGlob(pattern, s string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	matched, err := filepath.Match(pattern, s)
	if err != nil {
		return strings.Contains(s, pattern)
	}
	return matched
}

// matchGlobOrRegex detects whether the pattern is a regex and routes accordingly.
func matchGlobOrRegex(pattern, s string) bool {
	if looksLikeRegex(pattern) {
		return matchRegex(pattern, s)
	}
	if matchGlob(pattern, s) {
		return true
	}
	if !strings.ContainsAny(pattern, "*?[]") {
		return strings.Contains(s, pattern)
	}
	return false
}

// looksLikeRegex heuristically decides if a pattern should be treated as regex.
func looksLikeRegex(pattern string) bool {
	if len(pattern) > 0 && pattern[0] == '^' {
		return true
	}
	for _, c := range pattern {
		switch c {
		case '(', ')', '|', '+', '?', '$', '\\':
			return true
		}
	}
	return false
}

// matchRegex compiles the pattern as a regular expression and matches it.
func matchRegex(pattern, s string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return strings.Contains(s, pattern)
	}
	return re.MatchString(s)
}
