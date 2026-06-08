package gateway

import "strings"

// injectOffloadHints prepends pending background-tool notifications to the user message.
func injectOffloadHints(s *Server, sessionID, text string) string {
	mgr := s.toolOffloadMgr()
	if mgr == nil || sessionID == "" {
		return text
	}
	hints := mgr.PopResults(sessionID)
	if len(hints) == 0 {
		return text
	}
	var b strings.Builder
	for _, h := range hints {
		b.WriteString(h)
		b.WriteString("\n\n")
	}
	b.WriteString(text)
	return b.String()
}
