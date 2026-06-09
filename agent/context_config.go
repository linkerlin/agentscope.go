package agent

import (
	"strings"

	"github.com/linkerlin/agentscope.go/output"
)

// ContextConfig controls automatic context compression (aligned with PyV2 ContextConfig).
type ContextConfig struct {
	TriggerRatio      float64 // default 0.8
	ReserveRatio      float64 // default 0.1
	CompressionPrompt string
	SummaryTemplate   string
	ToolResultLimit   int
}

// CompressionSummary is the structured summary payload for context compression.
type CompressionSummary struct {
	TaskOverview         string `json:"task_overview"`
	CurrentState         string `json:"current_state"`
	ImportantDiscoveries string `json:"important_discoveries"`
	NextSteps            string `json:"next_steps"`
	ContextToPreserve    string `json:"context_to_preserve"`
}

// DefaultContextConfig returns PyV2-aligned defaults.
func DefaultContextConfig() ContextConfig {
	return ContextConfig{
		TriggerRatio: 0.8,
		ReserveRatio: 0.1,
		CompressionPrompt: "<system-hint>You have been working on the task described above " +
			"but have not yet completed it. " +
			"Now write a continuation summary that will allow you to resume " +
			"work efficiently in a future context window where the " +
			"conversation history will be replaced with this summary. " +
			"Your summary should be structured, concise, and actionable." +
			"</system-hint>",
		SummaryTemplate: "<system-info>Here is a summary of your previous work\n" +
			"# Task Overview\n" +
			"{task_overview}\n\n" +
			"# Current State\n" +
			"{current_state}\n\n" +
			"# Important Discoveries\n" +
			"{important_discoveries}\n\n" +
			"# Next Steps\n" +
			"{next_steps}\n\n" +
			"# Context to Preserve\n" +
			"{context_to_preserve}" +
			"</system-info>",
		ToolResultLimit: 3000,
	}
}

// DefaultSummarySchema returns the JSON schema for CompressionSummary.
func DefaultSummarySchema() *output.JSONSchema {
	return &output.JSONSchema{
		Type: "object",
		Properties: map[string]*output.SchemaProp{
			"task_overview": {
				Type:        "string",
				Description: "The user's core request and success criteria.",
			},
			"current_state": {
				Type:        "string",
				Description: "What has been completed so far.",
			},
			"important_discoveries": {
				Type:        "string",
				Description: "Technical constraints, decisions, and errors resolved.",
			},
			"next_steps": {
				Type:        "string",
				Description: "Specific actions needed to complete the task.",
			},
			"context_to_preserve": {
				Type:        "string",
				Description: "User preferences and domain-specific details to preserve.",
			},
		},
		Required: []string{
			"task_overview",
			"current_state",
			"important_discoveries",
			"next_steps",
			"context_to_preserve",
		},
	}
}

// FormatCompressionSummary renders the summary using SummaryTemplate.
func (c ContextConfig) FormatCompressionSummary(s CompressionSummary) string {
	tpl := c.SummaryTemplate
	if tpl == "" {
		tpl = DefaultContextConfig().SummaryTemplate
	}
	out := tpl
	repl := map[string]string{
		"{task_overview}":          s.TaskOverview,
		"{current_state}":          s.CurrentState,
		"{important_discoveries}":  s.ImportantDiscoveries,
		"{next_steps}":             s.NextSteps,
		"{context_to_preserve}":    s.ContextToPreserve,
	}
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}
