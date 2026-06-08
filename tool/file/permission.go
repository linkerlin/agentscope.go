package file

import "github.com/linkerlin/agentscope.go/tool"

func matchFilePathRule(pattern string, input map[string]any, key string) bool {
	path, _ := input[key].(string)
	return tool.MatchPathGlob(pattern, path)
}

func suggestFilePathRule(toolName string, input map[string]any, key string) []tool.SuggestedRule {
	path, _ := input[key].(string)
	return tool.SuggestPathParentGlob(toolName, path)
}

func (r *ReadFileTool) MatchRule(pattern string, input map[string]any) bool {
	return matchFilePathRule(pattern, input, "file_path")
}

func (r *ReadFileTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	return suggestFilePathRule(r.Name(), input, "file_path")
}

func (l *ListDirectoryTool) MatchRule(pattern string, input map[string]any) bool {
	return matchFilePathRule(pattern, input, "dir_path")
}

func (l *ListDirectoryTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	return suggestFilePathRule(l.Name(), input, "dir_path")
}

func (w *WriteFileTool) MatchRule(pattern string, input map[string]any) bool {
	return matchFilePathRule(pattern, input, "file_path")
}

func (w *WriteFileTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	return suggestFilePathRule(w.Name(), input, "file_path")
}

func (i *InsertTextFileTool) MatchRule(pattern string, input map[string]any) bool {
	return matchFilePathRule(pattern, input, "file_path")
}

func (i *InsertTextFileTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	return suggestFilePathRule(i.Name(), input, "file_path")
}

func (e *EditFileTool) MatchRule(pattern string, input map[string]any) bool {
	return matchFilePathRule(pattern, input, "file_path")
}

func (e *EditFileTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	return suggestFilePathRule(e.Name(), input, "file_path")
}

func (g *GlobTool) MatchRule(pattern string, input map[string]any) bool {
	return tool.MatchGlobSearch(pattern, input)
}

func (g *GlobTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	path, _ := input["path"].(string)
	return tool.SuggestGlobSearch(g.Name(), path)
}

func (g *GrepTool) MatchRule(pattern string, input map[string]any) bool {
	path, _ := input["path"].(string)
	return tool.MatchGrepPath(pattern, path)
}

func (g *GrepTool) GenerateSuggestions(input map[string]any) []tool.SuggestedRule {
	path, _ := input["path"].(string)
	return tool.SuggestGrepPath(g.Name(), path)
}
