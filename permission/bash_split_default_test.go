//go:build !treesitter

package permission

import "testing"

func TestParserBackend_Default(t *testing.T) {
	if ParserBackend() != "heuristic" {
		t.Fatalf("backend=%q", ParserBackend())
	}
}
