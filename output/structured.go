package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// StructuredRunner 使用 ChatModel 生成符合 schema 的 JSON 并解析到 target
type StructuredRunner struct {
	Model model.ChatModel
}

// Run 单次调用：将 schema 注入 system 提示，解析助手回复中的 JSON
func (r *StructuredRunner) Run(ctx context.Context, userText string, schema *JSONSchema, target any) error {
	if r.Model == nil {
		return errors.New("output: nil model")
	}
	if schema == nil {
		return errors.New("output: nil schema")
	}
	schemaBytes, _ := json.MarshalIndent(schema, "", "  ")
	sys := "You must respond with a single JSON object only, no markdown fence, matching this JSON Schema:\n" + string(schemaBytes)
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(sys).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(userText).Build(),
	}
	resp, err := r.Model.Chat(ctx, msgs)
	if err != nil {
		return err
	}
	return ParseJSONFromAssistant(resp.GetTextContent(), target)
}

// ParseJSONFromAssistant 从助手文本中提取 JSON（去除可选 ```json 围栏）
func ParseJSONFromAssistant(text string, target any) error {
	s := strings.TrimSpace(text)
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 && j < len(s) {
		s = s[:j+1]
	}
	return json.Unmarshal([]byte(s), target)
}

// SelfCorrectingParser 在解析失败时让模型修正
type SelfCorrectingParser struct {
	Model      model.ChatModel
	MaxRetries int
}

// ParseWithCorrection 尝试解析 raw；失败则将错误反馈给模型重试
func (p *SelfCorrectingParser) ParseWithCorrection(ctx context.Context, raw string, schema *JSONSchema, target any) error {
	if p.Model == nil {
		return errors.New("output: nil model")
	}
	max := p.MaxRetries
	if max < 1 {
		max = 2
	}
	schemaBytes, _ := json.MarshalIndent(schema, "", "  ")
	var lastErr error
	for attempt := 0; attempt < max; attempt++ {
		lastErr = ParseJSONFromAssistant(raw, target)
		if lastErr == nil {
			return nil
		}
		if attempt == max-1 {
			break
		}
		fix := fmt.Sprintf("Your previous output failed JSON parse: %v\nOriginal:\n%s\nSchema:\n%s\nReturn corrected JSON only.", lastErr, raw, string(schemaBytes))
		msgs := []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent(fix).Build(),
		}
		resp, err2 := p.Model.Chat(ctx, msgs)
		if err2 != nil {
			return err2
		}
		raw = resp.GetTextContent()
	}
	return lastErr
}
