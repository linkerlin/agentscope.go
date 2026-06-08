package tool

// PermissionDecision is the outcome of a tool-level permission check.
type PermissionDecision string

const (
	PermAllow       PermissionDecision = "allow"
	PermDeny        PermissionDecision = "deny"
	PermAsk         PermissionDecision = "ask"
	PermPassthrough PermissionDecision = "passthrough"
)

// SuggestedRule is a tool-level permission rule suggestion.
type SuggestedRule struct {
	Name     string
	ToolName string
	Target   string
	Pattern  string
	Decision PermissionDecision
}

// PermissionDecider allows tools to participate in permission evaluation (PyV2 check_permissions).
// Return passthrough=true to defer to the engine's rule-based logic.
type PermissionDecider interface {
	Tool
	CheckPermissions(input map[string]any, ctx any) (decision PermissionDecision, message, reason string, passthrough bool)
}

// RuleMatcher allows tools to match fine-grained permission rules (PyV2 match_rule).
// An empty pattern matches all invocations of the tool.
type RuleMatcher interface {
	Tool
	MatchRule(pattern string, input map[string]any) bool
}

// SuggestionGenerator allows tools to suggest permission rules (PyV2 generate_suggestions).
type SuggestionGenerator interface {
	Tool
	GenerateSuggestions(input map[string]any) []SuggestedRule
}
