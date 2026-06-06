// Package permission provides a rule-based permission engine for agent tool execution.
// It supports glob path matching, command substring matching, and HITL (Human-in-the-Loop)
// suspend-resume via the ASK decision.
package permission

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
)

// Mode defines the default behavior when no explicit rule matches.
type Mode string

const (
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
	Target   string   `json:"target"`   // "tool_name" | "file_path" | "command"
	Pattern  string   `json:"pattern"`  // glob for file_path, substring for command/tool_name
	Decision Decision `json:"decision"` // allow | deny | ask | passthrough
}

// Engine evaluates permission rules against tool calls.
type Engine struct {
	Mode  Mode
	Rules []Rule
}

// NewEngine creates a permission engine with the given mode and rules.
func NewEngine(mode Mode, rules []Rule) *Engine {
	if mode == "" {
		mode = ModeExplore
	}
	return &Engine{Mode: mode, Rules: rules}
}

// Evaluation holds the result for a single tool call.
type Evaluation struct {
	ToolCallID string
	ToolName   string
	Decision   Decision
	Reason     string
}

// Evaluate runs the permission engine against a slice of tool calls.
// It returns a per-tool-call decision. If any decision is ASK, the caller
// (typically the agent) should suspend and emit a RequireUserConfirmEvent.
func (e *Engine) Evaluate(toolCalls []*message.ToolUseBlock) ([]Evaluation, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}

	results := make([]Evaluation, len(toolCalls))
	for i, tc := range toolCalls {
		decision, reason := e.evaluateOne(tc)
		results[i] = Evaluation{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Decision:   decision,
			Reason:     reason,
		}
	}
	return results, nil
}

func (e *Engine) evaluateOne(tc *message.ToolUseBlock) (Decision, string) {
	// Extract target values from tool call arguments.
	filePath := extractString(tc.Input, "file_path", "dir_path", "path")
	command := extractString(tc.Input, "command")

	for _, rule := range e.Rules {
		matched := false
		switch rule.Target {
		case "tool_name":
			matched = matchGlob(rule.Pattern, tc.Name)
		case "file_path":
			matched = matchGlob(rule.Pattern, filePath)
		case "command":
			matched = matchGlobOrRegex(rule.Pattern, command)
		case "regex":
			matched = matchRegex(rule.Pattern, tc.Name+" "+filePath+" "+command)
		default:
			matched = matchGlobOrRegex(rule.Pattern, tc.Name)
		}
		if matched {
			return rule.Decision, fmt.Sprintf("matched rule %q", rule.Name)
		}
	}

	// No rule matched — fall back to mode default.
	return e.defaultDecision(tc), "no matching rule"
}

func (e *Engine) defaultDecision(tc *message.ToolUseBlock) Decision {
	switch e.Mode {
	case ModeBypass:
		return DecisionAllow
	case ModeDontAsk:
		return DecisionAllow
	case ModeExplore:
		// In explore mode, deny write operations unless explicitly allowed.
		if isWriteTool(tc.Name) {
			return DecisionAsk
		}
		return DecisionAllow
	case ModeAcceptEdits:
		if isEditTool(tc.Name) {
			return DecisionAllow
		}
		if isWriteTool(tc.Name) {
			return DecisionAsk
		}
		return DecisionAllow
	default:
		return DecisionAsk
	}
}

func isWriteTool(name string) bool {
	switch name {
	case "write_text_file", "insert_text_file", "execute_shell_command":
		return true
	}
	return false
}

func isEditTool(name string) bool {
	switch name {
	case "write_text_file", "insert_text_file":
		return true
	}
	return false
}

func extractString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

// matchGlob performs a simple glob match. It uses filepath.Match for full
// glob support. A pattern of "*" matches everything.
func matchGlob(pattern, s string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	matched, err := filepath.Match(pattern, s)
	if err != nil {
		// Invalid pattern — fall back to substring.
		return strings.Contains(s, pattern)
	}
	return matched
}

// matchGlobOrRegex detects whether the pattern is a regex (starts with "^" or
// contains regex metacharacters like ".*" or "$") and routes accordingly.
// For non-regex, non-glob patterns it falls back to substring matching so that
// e.g. "ls" matches "ls -la".
func matchGlobOrRegex(pattern, s string) bool {
	if looksLikeRegex(pattern) {
		return matchRegex(pattern, s)
	}
	if matchGlob(pattern, s) {
		return true
	}
	// If the pattern contains no glob wildcards, also try substring.
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
		// Invalid regex — fall back to substring.
		return strings.Contains(s, pattern)
	}
	return re.MatchString(s)
}
