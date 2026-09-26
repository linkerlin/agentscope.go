package console

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	userStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	agentStyle    = lipgloss.NewStyle()
	thinkingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	toolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	confirmStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func styleHeader(s string) string        { return headerStyle.Render(s) }
func styleDim(s string) string           { return dimStyle.Render(s) }
func styleAgent(s string) string         { return agentStyle.Render(s) }
func styleThinking(s string) string      { return thinkingStyle.Render("thinking: " + s) }
func styleHint(s string) string          { return hintStyle.Render("hint: " + s) }
func styleToolLine(s string) string      { return toolStyle.Render(s) }
func styleConfirm(s string) string       { return confirmStyle.Render(s) }
func styleError(s string) string         { return errorStyle.Render(s) }
func styleUser(name, text string) string { return userStyle.Render(name+"> ") + text }

func styleToolResultBody(body string) string {
	return toolStyle.Render(indent(body))
}

// compactArgs renders streaming tool-call arguments compactly: valid JSON is
// re-encoded through compactInput; anything else (partial/truncated deltas)
// falls back to a truncated raw string.
func compactArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(raw), &in); err == nil {
		return compactInput(in)
	}
	if len(raw) > 120 {
		raw = raw[:117] + "..."
	}
	return raw
}

// diffStats counts added/removed lines when body looks like a unified diff.
// The marker check (hunk header or git header) keeps ordinary text containing
// leading +/- characters from being reported as a diff.
func diffStats(body string) (added, removed int, ok bool) {
	hasMarker := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "diff --git") {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return 0, 0, false
	}
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			removed++
		}
	}
	return added, removed, true
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

// truncateLines caps s to max lines (max<0 means unlimited) and appends an
// ellipsis marker naming the number of dropped lines.
func truncateLines(s string, max int) string {
	if max < 0 {
		return s
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[:max], "\n") + fmt.Sprintf("\n… (%d more lines)", len(lines)-max)
}

func verbosityName(v Verbosity) string {
	switch v {
	case Quiet:
		return "quiet"
	case Debug:
		return "debug"
	default:
		return "default"
	}
}
