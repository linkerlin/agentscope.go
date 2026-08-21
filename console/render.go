package console

import (
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

func styleToolResult(name, body string) string {
	return toolStyle.Render(fmt.Sprintf("· result (%s):\n%s", name, indent(body)))
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
