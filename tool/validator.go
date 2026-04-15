package tool

import (
	"fmt"

	"github.com/linkerlin/agentscope.go/message"
)

// ValidateToolResultMatch validates that ToolResult messages match pending ToolUse blocks.
func ValidateToolResultMatch(assistantMsg *message.Msg, inputMsgs []*message.Msg) error {
	if assistantMsg == nil {
		return nil
	}
	pendingToolUses := assistantMsg.GetToolUseCalls()
	if len(pendingToolUses) == 0 {
		return nil
	}
	if len(inputMsgs) == 0 {
		names := make([]string, 0, len(pendingToolUses))
		for _, tu := range pendingToolUses {
			names = append(names, tu.Name)
		}
		return fmt.Errorf("cannot proceed without ToolResult when there are pending ToolUse. Pending tools: %v", names)
	}
	providedIDs := make(map[string]struct{})
	for _, m := range inputMsgs {
		for _, tr := range m.GetToolResults() {
			providedIDs[tr.ToolUseID] = struct{}{}
		}
	}
	var missing []string
	for _, tu := range pendingToolUses {
		if _, ok := providedIDs[tu.ID]; !ok {
			missing = append(missing, tu.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing ToolResult for pending ToolUse. Missing IDs: %v", missing)
	}
	return nil
}
