//go:build !treesitter

package permission

func parserBackendName() string { return "heuristic" }

func splitCompoundCommandImpl(cmd string) []string {
	return splitCompoundCommandHeuristic(cmd)
}
