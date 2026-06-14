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
	Model      model.ChatModel
	MaxRetries int // 默认 0 表示不自动纠正；>0 时启用 SelfCorrectingParser
}

// Run 单次调用：将 schema 注入 system 提示，解析助手回复中的 JSON
// 若 MaxRetries > 0 且解析失败，会自动将错误反馈给模型要求修正。
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
	raw := resp.GetTextContent()
	if r.MaxRetries > 0 {
		parser := &SelfCorrectingParser{Model: r.Model, MaxRetries: r.MaxRetries}
		return parser.ParseWithCorrection(ctx, raw, schema, target)
	}
	return ParseJSONFromAssistant(raw, target)
}

// StreamResult represents a partial or final result from RunStream.
type StreamResult struct {
	Partial map[string]any // latest successfully parsed partial JSON
	Done    bool           // true when the stream is complete
	Err     error          // non-nil on error
}

// RunStream calls the model in streaming mode and emits partial JSON results
// as chunks arrive. The final result is also parsed into target.
//
// The channel closes after the final result (Done=true) or an error.
func (r *StructuredRunner) RunStream(ctx context.Context, userText string, schema *JSONSchema, target any) (<-chan StreamResult, error) {
	if r.Model == nil {
		return nil, errors.New("output: nil model")
	}
	if schema == nil {
		return nil, errors.New("output: nil schema")
	}

	schemaBytes, _ := json.MarshalIndent(schema, "", "  ")
	sys := "You must respond with a single JSON object only, no markdown fence, matching this JSON Schema:\n" + string(schemaBytes)
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent(sys).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent(userText).Build(),
	}

	ch, err := r.Model.ChatStream(ctx, msgs)
	if err != nil {
		return nil, err
	}

	out := make(chan StreamResult, 8)
	go func() {
		defer close(out)
		var sb strings.Builder
		for chunk := range ch {
			if chunk == nil {
				continue
			}
			if chunk.Done {
				break
			}
			if chunk.Delta == "" {
				continue
			}
			sb.WriteString(chunk.Delta)

			if partial := tryParsePartial(sb.String()); partial != nil {
				out <- StreamResult{Partial: partial}
			}
		}

		raw := sb.String()
		if err := ParseJSONFromAssistant(raw, target); err != nil {
			out <- StreamResult{Done: true, Err: fmt.Errorf("output: final JSON parse failed: %w", err)}
			return
		}
		final := tryParsePartial(raw)
		out <- StreamResult{Partial: final, Done: true}
	}()

	return out, nil
}

// tryParsePartial attempts to extract valid JSON from possibly-incomplete text.
// Returns nil if no valid JSON prefix can be parsed.
func tryParsePartial(s string) map[string]any {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
	} else {
		return nil
	}

	// Fast path: complete valid JSON
	if result := tryUnmarshalMap(s); result != nil {
		return result
	}

	// Try from first { to last } (handles markdown fences and trailing text)
	if j := strings.LastIndex(s, "}"); j > 0 {
		if result := tryUnmarshalMap(s[:j+1]); result != nil {
			return result
		}
	}

	// Partial JSON: trim at last comma boundary, then close open structures
	for trimmed := trimAtLastComma(s); trimmed != "" && trimmed != s; trimmed = trimAtLastComma(trimmed) {
		closed := closeOpenStructures(trimmed)
		if result := tryUnmarshalMap(closed); result != nil {
			return result
		}
	}
	return nil
}

func tryUnmarshalMap(s string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err == nil {
		return result
	}
	return nil
}

// trimAtLastComma returns the substring up to (not including) the last comma.
// Returns "" if no comma exists, or if the input is already shorter than before
// the last comma (prevents infinite loop).
func trimAtLastComma(s string) string {
	idx := strings.LastIndex(s, ",")
	if idx < 0 {
		return ""
	}
	return s[:idx]
}

// closeOpenStructures appends missing closing braces/brackets to make partial JSON valid.
func closeOpenStructures(s string) string {
	openBraces, openBrackets := 0, 0
	inString := false
	escaped := false
	for _, c := range s {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			openBraces++
		case '}':
			openBraces--
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		}
	}
	// Remove trailing incomplete value (e.g. partial string without closing quote)
	if inString {
		s += `"`
	}
	// Remove trailing comma
	s = strings.TrimRight(s, ", \t\n\r")
	// Close open structures
	for openBrackets > 0 {
		s += "]"
		openBrackets--
	}
	for openBraces > 0 {
		s += "}"
		openBraces--
	}
	return s
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
