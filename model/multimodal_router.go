package model

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// MultimodalRouter routes chat requests to different underlying models
// based on the presence of media content (images, audio, video) in the input messages.
//
// This is useful when working with vision models like qwen-vl-plus or gpt-4o,
// where you want to automatically switch from a text-only model to a vision-capable
// model when the user provides images or videos.
type MultimodalRouter struct {
	defaultModel ChatModel
	visionModel  ChatModel
}

// NewMultimodalRouter creates a router.
//   defaultModel: used when no media content is detected.
//   visionModel:  used when any message contains ImageBlock, AudioBlock, or VideoBlock.
func NewMultimodalRouter(defaultModel, visionModel ChatModel) *MultimodalRouter {
	return &MultimodalRouter{
		defaultModel: defaultModel,
		visionModel:  visionModel,
	}
}

func (r *MultimodalRouter) ModelName() string {
	if r.defaultModel != nil {
		return r.defaultModel.ModelName()
	}
	if r.visionModel != nil {
		return r.visionModel.ModelName()
	}
	return "multimodal-router"
}

func (r *MultimodalRouter) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	m := r.selectModel(messages)
	return m.Chat(ctx, messages, options...)
}

func (r *MultimodalRouter) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	m := r.selectModel(messages)
	return m.ChatStream(ctx, messages, options...)
}

func (r *MultimodalRouter) selectModel(messages []*message.Msg) ChatModel {
	if r.visionModel != nil && hasAnyMedia(messages) {
		return r.visionModel
	}
	return r.defaultModel
}

func hasAnyMedia(messages []*message.Msg) bool {
	for _, msg := range messages {
		for _, block := range msg.Content {
			switch block.(type) {
			case *message.ImageBlock, *message.AudioBlock, *message.VideoBlock:
				return true
			}
		}
	}
	return false
}
