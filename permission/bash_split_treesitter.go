//go:build treesitter

package permission

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
)

func parserBackendName() string { return "treesitter" }

func splitCompoundCommandImpl(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	parser := sitter.NewParser()
	parser.SetLanguage(bash.GetLanguage())
	tree := parser.Parse(nil, []byte(cmd))
	if tree == nil {
		return splitCompoundCommandHeuristic(cmd)
	}
	root := tree.RootNode()
	if root == nil || root.HasError() {
		return splitCompoundCommandHeuristic(cmd)
	}
	parts := extractBashCommands(root, cmd)
	if len(parts) == 0 {
		return splitCompoundCommandHeuristic(cmd)
	}
	return parts
}

func extractBashCommands(root *sitter.Node, src string) []string {
	var out []string
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "command", "redirected_command", "declaration_command":
			out = append(out, src[n.StartByte():n.EndByte()])
		case "subshell":
			out = append(out, src[n.StartByte():n.EndByte()])
		case "list", "pipeline", "command_list":
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				switch child.Type() {
				case "&&", "||", ";", "|", "|&":
					continue
				default:
					walk(child)
				}
			}
		default:
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
		}
	}
	walk(root)
	return out
}
