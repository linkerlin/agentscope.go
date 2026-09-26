package react

import (
	"fmt"

	"github.com/linkerlin/agentscope.go/message"
)

// limitImages caps image blocks sent to the model per ContextConfig.MaxImageNum
// (0 = unlimited). Surplus images become text placeholders so visual token
// usage stays bounded (PyV2 ContextConfig.max_image_num parity). The stored
// messages are never mutated: only the returned slice carries replacements.
func limitImages(msgs []*message.Msg, max int) []*message.Msg {
	if max <= 0 || len(msgs) == 0 {
		return msgs
	}
	count := 0
	out := make([]*message.Msg, len(msgs))
	for i, m := range msgs {
		if m == nil || !hasImageContent(m.Content) {
			out[i] = m
			continue
		}
		blocks := make([]message.ContentBlock, len(m.Content))
		for j, b := range m.Content {
			if isImageContent(b) {
				count++
				if count > max {
					blocks[j] = message.NewTextBlock(fmt.Sprintf(
						"[Image omitted: context image limit of %d reached]", max))
					continue
				}
			}
			blocks[j] = b
		}
		cp := *m
		cp.Content = blocks
		out[i] = &cp
	}
	return out
}

func hasImageContent(blocks []message.ContentBlock) bool {
	for _, b := range blocks {
		if isImageContent(b) {
			return true
		}
	}
	return false
}

func isImageContent(b message.ContentBlock) bool {
	switch v := b.(type) {
	case *message.ImageBlock:
		return true
	case *message.DataBlock:
		return v.Source != nil && v.BlockType() == message.TypeImage
	}
	return false
}
