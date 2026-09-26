package formatter

import (
	"strings"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
)

// XAIFormatter formats messages for the xAI (Grok) API. Grok speaks the
// OpenAI chat format but only accepts JPEG and PNG images; other media is
// replaced with a text placeholder instead of being sent and rejected.
type XAIFormatter struct {
	*OpenAIFormatter
}

var _ Formatter = (*XAIFormatter)(nil)

func NewXAIFormatter() *XAIFormatter {
	return &XAIFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

// FormatMessages implements Formatter.
func (f *XAIFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	return f.FormatMessagesTyped(msgs), nil
}

// FormatMessagesTyped converts agent Msgs like OpenAIFormatter, then filters
// media parts down to what the xAI API accepts.
func (f *XAIFormatter) FormatMessagesTyped(msgs []*message.Msg) []goopenai.ChatCompletionMessage {
	out := f.OpenAIFormatter.FormatMessagesTyped(msgs)
	for i := range out {
		out[i].MultiContent = filterXAIMediaParts(out[i].MultiContent)
	}
	return out
}

// filterXAIMediaParts keeps text and JPEG/PNG image parts, replacing any
// other image with a text placeholder so the turn still carries a trace of
// the dropped media.
func filterXAIMediaParts(parts []goopenai.ChatMessagePart) []goopenai.ChatMessagePart {
	if len(parts) == 0 {
		return parts
	}
	filtered := make([]goopenai.ChatMessagePart, 0, len(parts))
	for _, p := range parts {
		if p.Type != goopenai.ChatMessagePartTypeImageURL || p.ImageURL == nil {
			filtered = append(filtered, p)
			continue
		}
		if xaiSupportedImage(p.ImageURL.URL) {
			filtered = append(filtered, p)
			continue
		}
		filtered = append(filtered, goopenai.ChatMessagePart{
			Type: goopenai.ChatMessagePartTypeText,
			Text: "[Image omitted: xAI only supports JPEG and PNG images]",
		})
	}
	return filtered
}

// xaiSupportedImage reports whether an image URL is acceptable to the xAI API.
// Data URLs are checked by MIME type; remote URLs are passed through since
// their type cannot be known without fetching.
func xaiSupportedImage(url string) bool {
	if !strings.HasPrefix(url, "data:") {
		return true
	}
	rest := strings.TrimPrefix(url, "data:")
	mime := rest
	if i := strings.Index(rest, ";"); i >= 0 {
		mime = rest[:i]
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	return mime == "image/jpeg" || mime == "image/png" || mime == "image/jpg"
}
