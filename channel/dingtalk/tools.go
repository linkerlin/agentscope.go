package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// wikiNode is one DingTalk knowledge-base (wiki) node returned by the search
// and detail APIs.
type wikiNode struct {
	NodeToken   string `json:"nodeToken"`
	Title       string `json:"title"`
	ParentToken string `json:"parentToken"`
	SpaceID     string `json:"spaceId"`
	Type        string `json:"type"` // "wiki" | "doc" | ...
	// DocURL is the reader link when exposed by the API.
	DocURL string `json:"fullScreenUrl"`
}

// WikiSearchTool lets the agent search a DingTalk knowledge base by keyword.
// Docs: POST /v2.0/dingtalk/wiki/nodes/search
type WikiSearchTool struct {
	Ch *Channel
}

// Name returns the tool name.
func (t *WikiSearchTool) Name() string { return "dingtalk_wiki_search" }

// Description returns the tool description.
func (t *WikiSearchTool) Description() string {
	return "Search a DingTalk wiki knowledge base for nodes matching a keyword. Returns node tokens and titles."
}

// Spec returns the tool JSON schema.
func (t *WikiSearchTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{"type": "string", "description": "Search keyword"},
				"limit":   map[string]any{"type": "integer", "description": "Max results (default 10)"},
			},
			"required": []string{"keyword"},
		},
	}
}

// Execute runs the wiki search.
func (t *WikiSearchTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	keyword, _ := input["keyword"].(string)
	if keyword == "" {
		return tool.NewTextResponse("dingtalk_wiki_search: keyword is required"), nil
	}
	limit := 10
	if n, ok := input["limit"].(float64); ok && n > 0 {
		limit = int(n)
	}
	token, err := t.Ch.accessToken(ctx)
	if err != nil {
		return tool.NewTextResponse("dingtalk_wiki_search failed: " + err.Error()), nil
	}
	body, _ := json.Marshal(map[string]any{"keyword": keyword, "maxResults": limit})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.Ch.apiBase+"/v2.0/dingtalk/wiki/nodes/search?access_token="+token, bytes.NewReader(body))
	if err != nil {
		return tool.NewTextResponse("dingtalk_wiki_search failed: " + err.Error()), nil
	}
	req.Header.Set("Content-Type", "application/json")
	var out struct {
		Nodes []wikiNode `json:"result"`
	}
	if err := t.Ch.doJSON(req, &out); err != nil {
		return tool.NewTextResponse("dingtalk_wiki_search failed: " + err.Error()), nil
	}
	if len(out.Nodes) == 0 {
		return tool.NewTextResponse("no wiki nodes matched"), nil
	}
	text := fmt.Sprintf("%d wiki node(s):", len(out.Nodes))
	for _, n := range out.Nodes {
		text += fmt.Sprintf("\n- %s (node_token=%s type=%s)", n.Title, n.NodeToken, n.Type)
	}
	return tool.NewTextResponse(text), nil
}

// WikiNodeTool fetches one wiki node's detail by node token.
// Docs: GET /v2.0/dingtalk/wiki/nodes/{nodeToken}
type WikiNodeTool struct {
	Ch *Channel
}

// Name returns the tool name.
func (t *WikiNodeTool) Name() string { return "dingtalk_wiki_node" }

// Description returns the tool description.
func (t *WikiNodeTool) Description() string {
	return "Fetch one DingTalk wiki node's details (title, type, doc URL) by node token."
}

// Spec returns the tool JSON schema.
func (t *WikiNodeTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_token": map[string]any{"type": "string", "description": "Wiki node token"},
			},
			"required": []string{"node_token"},
		},
	}
}

// Execute fetches the node detail.
func (t *WikiNodeTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	nodeToken, _ := input["node_token"].(string)
	if nodeToken == "" {
		return tool.NewTextResponse("dingtalk_wiki_node: node_token is required"), nil
	}
	token, err := t.Ch.accessToken(ctx)
	if err != nil {
		return tool.NewTextResponse("dingtalk_wiki_node failed: " + err.Error()), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		t.Ch.apiBase+"/v2.0/dingtalk/wiki/nodes/"+nodeToken+"?access_token="+token, nil)
	if err != nil {
		return tool.NewTextResponse("dingtalk_wiki_node failed: " + err.Error()), nil
	}
	var n wikiNode
	if err := t.Ch.doJSON(req, &n); err != nil {
		return tool.NewTextResponse("dingtalk_wiki_node failed: " + err.Error()), nil
	}
	text := fmt.Sprintf("title: %s\ntype: %s\nspace: %s\ndoc_url: %s", n.Title, n.Type, n.SpaceID, n.DocURL)
	return tool.NewTextResponse(text), nil
}

// RegisterTools returns the DingTalk wiki tools bound to the channel.
func RegisterTools(ch *Channel) []tool.Tool {
	return []tool.Tool{&WikiSearchTool{Ch: ch}, &WikiNodeTool{Ch: ch}}
}
