package controlplane

import "strings"

// Visibility is the public/private boundary of an artifact. LoopX discipline:
// what may be OBSERVED (public projection) vs what must stay INTERNAL. Distinct
// from service/access policy (which governs CROSS-USER resource sharing) —
// this governs the agent-internal partition of reasoning/evidence.
type Visibility string

const (
	// VisibilityPublic may appear in a public projection (board, kanban, log).
	VisibilityPublic Visibility = "public"
	// VisibilityPrivate must stay in ignored local state; never committed or
	// projected without redaction.
	VisibilityPrivate Visibility = "private"
)

// DefaultVisibility is applied when an artifact does not declare one. The safe
// default is PRIVATE: never leak by accident.
const DefaultVisibility Visibility = VisibilityPrivate

// RedactEvidence returns a public-safe copy of e: private source references and
// locally-pathed artifacts are stripped or generalized before the evidence may
// flow to a public/multi-user projection. The integrity of the evidence KIND
// and SUMMARY is preserved so the lineage stays auditable.
//
// ponytail: heuristic redaction — strips anything that looks like an absolute
// path, a file:// URL, or a private marker. Tighten with a real schema allowlist
// when a sink requires stronger guarantees.
func RedactEvidence(e Evidence) Evidence {
	out := e
	out.SourceRef = redactSourceRef(e.SourceRef)
	out.Summary = redactPrivateMarkers(e.Summary)
	return out
}

// RedactEvidenceSlice redacts a slice in place-safe fashion (returns a new slice).
func RedactEvidenceSlice(ev []Evidence) []Evidence {
	out := make([]Evidence, len(ev))
	for i, e := range ev {
		out[i] = RedactEvidence(e)
	}
	return out
}

// redactSourceRef blanks refs that point at local/private locations.
func redactSourceRef(ref string) string {
	if ref == "" {
		return ""
	}
	r := ref
	switch {
	case strings.HasPrefix(r, "file://"),
		strings.HasPrefix(r, "/"),
		strings.HasPrefix(r, "\\"), // Windows abs
		strings.Contains(r, ":\\"), // Windows drive
		strings.HasPrefix(strings.ToLower(r), "file:"):
		return "(redacted:local)"
	}
	if strings.Contains(strings.ToLower(r), "private") || strings.Contains(strings.ToLower(r), "secret") {
		return "(redacted:private)"
	}
	return r
}

// redactPrivateMarkers scrubs inline private markers from a summary.
func redactPrivateMarkers(s string) string {
	if s == "" {
		return ""
	}
	// Replace common local-path leak patterns with a placeholder.
	redacted := s
	for _, marker := range []string{"file://", "C:\\", "D:\\"} {
		if strings.Contains(redacted, marker) {
			redacted = "(redacted)"
			break
		}
	}
	return redacted
}

// EnforceVisibility reports whether an artifact of the given visibility may be
// projected to a public sink. Private artifacts must be Redact-ed first or kept
// in owner-only state.
func EnforceVisibility(v Visibility) bool {
	if v == "" {
		v = DefaultVisibility
	}
	return v == VisibilityPublic
}
