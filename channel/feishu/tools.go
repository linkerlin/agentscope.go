package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// SendMessageTool lets an agent send a text message to a Feishu chat.
// Mirrors Python's Feishu SendMessage tool.
type SendMessageTool struct {
	Channel *Channel
}

// Name returns the tool name.
func (t *SendMessageTool) Name() string { return "feishu_send_message" }

// Description returns the tool description.
func (t *SendMessageTool) Description() string {
	return "Send a text message to a Feishu chat (by chat_id)."
}

// Spec returns the tool JSON schema.
func (t *SendMessageTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chat_id": map[string]any{"type": "string", "description": "Target Feishu chat id"},
				"text":    map[string]any{"type": "string", "description": "Message text to send"},
			},
			"required": []string{"chat_id", "text"},
		},
	}
}

// Execute sends the text message via the channel.
func (t *SendMessageTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	chatID, _ := input["chat_id"].(string)
	text, _ := input["text"].(string)
	if chatID == "" || text == "" {
		return tool.NewTextResponse("feishu_send_message: chat_id and text are required"), nil
	}
	if err := t.Channel.SendText(ctx, chatID, text); err != nil {
		return tool.NewTextResponse("feishu_send_message failed: " + err.Error()), nil
	}
	return tool.NewTextResponse("Sent."), nil
}

// ListChatsTool lets an agent list the chats the bot belongs to.
// Mirrors Python's Feishu ListChats tool.
type ListChatsTool struct {
	Channel *Channel
}

// Name returns the tool name.
func (t *ListChatsTool) Name() string { return "feishu_list_chats" }

// Description returns the tool description.
func (t *ListChatsTool) Description() string {
	return "List the Feishu chats the bot belongs to (chat_id + name)."
}

// Spec returns the tool JSON schema.
func (t *ListChatsTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

// Execute lists chats via GET /im/v1/chats.
func (t *ListChatsTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	tok, err := t.Channel.token(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		t.Channel.baseURL+"/im/v1/chats?page_size=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := t.Channel.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("feishu: list chats: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var data struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ChatID string `json:"chat_id"`
				Name   string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.Code != 0 {
		return nil, fmt.Errorf("feishu: list chats code=%d", data.Code)
	}
	out := ""
	for _, item := range data.Data.Items {
		out += fmt.Sprintf("- %s (%s)\n", item.Name, item.ChatID)
	}
	if out == "" {
		return tool.NewTextResponse("No chats found."), nil
	}
	return tool.NewTextResponse(out), nil
}

// compile-time assertions.
var (
	_ tool.Tool = (*SendMessageTool)(nil)
	_ tool.Tool = (*ListChatsTool)(nil)
)
