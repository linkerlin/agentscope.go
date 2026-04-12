package react

import (
	"context"
	"errors"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/output"
)

// CallStructured 单次结构化输出：将用户消息文本与 JSON Schema 交给模型，解析到 target 指针
func (a *ReActAgent) CallStructured(ctx context.Context, user *message.Msg, schema *output.JSONSchema, target any) error {
	if user == nil {
		return errors.New("react agent: nil user message")
	}
	r := &output.StructuredRunner{Model: a.chatModel}
	return r.Run(ctx, user.GetTextContent(), schema, target)
}
