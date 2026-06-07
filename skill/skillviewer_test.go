package skill

import (
	"context"
	"testing"
)

func TestSkillViewerTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&AgentSkill{Name: "data-analysis", SkillContent: "# Analysis\nSteps..."})
	r.SetActive("data-analysis_custom", true)

	tool := NewSkillViewerTool(r)
	resp, err := tool.Execute(context.Background(), map[string]any{"skill": "data-analysis"})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
}
