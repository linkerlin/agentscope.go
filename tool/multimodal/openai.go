package multimodal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	oai "github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/tool"
)

// OpenAIMultiModalTool wraps OpenAI multimodal APIs (DALL-E, vision) as tools.
type OpenAIMultiModalTool struct {
	apiKey    string
	baseURL   string
	client    *goopenai.Client
	chatModel model.ChatModel
}

// NewOpenAIMultiModalTool creates a tool with the given OpenAI API key.
func NewOpenAIMultiModalTool(apiKey string) (*OpenAIMultiModalTool, error) {
	return NewOpenAIMultiModalToolWithBaseURL(apiKey, "")
}

// NewOpenAIMultiModalToolWithBaseURL creates a tool with a custom base URL.
func NewOpenAIMultiModalToolWithBaseURL(apiKey, baseURL string) (*OpenAIMultiModalTool, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("openai: API key is required")
	}
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := goopenai.NewClientWithConfig(cfg)

	ob := oai.Builder().APIKey(apiKey)
	if baseURL != "" {
		ob.BaseURL(baseURL)
	}
	chatModel, err := ob.Build()
	if err != nil {
		return nil, fmt.Errorf("openai multimodal: failed to build chat model: %w", err)
	}

	return &OpenAIMultiModalTool{
		apiKey:    apiKey,
		baseURL:   baseURL,
		client:    client,
		chatModel: chatModel,
	}, nil
}

// NewOpenAIMultiModalToolWithClient creates a tool with an injected HTTP client
// and optional chat model (for testing).
func NewOpenAIMultiModalToolWithClient(apiKey, baseURL string, httpClient *http.Client, chatModel model.ChatModel) *OpenAIMultiModalTool {
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}
	client := goopenai.NewClientWithConfig(cfg)
	return &OpenAIMultiModalTool{
		apiKey:    apiKey,
		baseURL:   baseURL,
		client:    client,
		chatModel: chatModel,
	}
}

// TextToImageTool returns a tool that generates images via DALL-E.
func (m *OpenAIMultiModalTool) TextToImageTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text prompt to generate image",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "The model to use, e.g., 'dall-e-3', 'dall-e-2'",
			},
			"n": map[string]any{
				"type":        "integer",
				"description": "The number of images to generate (1 for dall-e-3, 1-10 for dall-e-2)",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "Size of the image, e.g., '1024x1024', '1792x1024', '1024x1792'",
			},
			"quality": map[string]any{
				"type":        "string",
				"description": "The quality of the image ('standard' or 'hd' for dall-e-3)",
			},
			"response_format": map[string]any{
				"type":        "string",
				"description": "The format of the response ('url' or 'b64_json')",
			},
		},
		"required": []string{"prompt"},
	}

	return tool.NewFunctionTool(
		"openai_text_to_image",
		"Generate image(s) based on the given prompt using OpenAI DALL-E models. Returns image URL(s) or base64 data.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			prompt, _ := input["prompt"].(string)
			if strings.TrimSpace(prompt) == "" {
				return tool.NewErrorResponse(errors.New("prompt is required")), nil
			}

			modelName := defaultString(input["model"], "dall-e-3")
			n := defaultInt(input["n"], 1)
			size := defaultString(input["size"], "1024x1024")
			quality := defaultString(input["quality"], "standard")
			respFormat := defaultString(input["response_format"], "url")

			req := goopenai.ImageRequest{
				Prompt:         prompt,
				Model:          modelName,
				N:              n,
				Size:           size,
				Quality:        quality,
				ResponseFormat: respFormat,
			}

			resp, err := m.client.CreateImage(ctx, req)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("openai image generation failed: %w", err)), nil
			}

			var blocks []message.ContentBlock
			for _, item := range resp.Data {
				if respFormat == "b64_json" && item.B64JSON != "" {
					blocks = append(blocks, message.NewDataBlock(message.TypeImage, &message.Source{
						Type:      message.SourceTypeBase64,
						MediaType: "image/png",
						Data:      item.B64JSON,
					}))
				} else if item.URL != "" {
					blocks = append(blocks, message.NewDataBlock(message.TypeImage, &message.Source{
						Type: message.SourceTypeURL,
						URL:  item.URL,
					}))
				}
			}

			if len(blocks) == 0 {
				return tool.NewErrorResponse(errors.New("no valid image content generated")), nil
			}

			return &tool.Response{Content: blocks, IsLast: true}, nil
		},
	)
}

// ImageToTextTool returns a vision tool that converts images to text.
func (m *OpenAIMultiModalTool) visionChatModel(modelName string) (model.ChatModel, error) {
	if modelName == "" {
		return m.chatModel, nil
	}
	if m.chatModel != nil && m.chatModel.ModelName() == modelName {
		return m.chatModel, nil
	}
	ob := oai.Builder().APIKey(m.apiKey).ModelName(modelName)
	if m.baseURL != "" {
		ob.BaseURL(m.baseURL)
	}
	return ob.Build()
}

func (m *OpenAIMultiModalTool) ImageToTextTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image_urls": map[string]any{
				"type":        "string",
				"description": "The URLs of the images to analyze (comma-separated)",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text prompt describing what to extract from the images",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "The vision model to use, e.g., 'gpt-4o', 'gpt-4-vision-preview'",
			},
			"max_tokens": map[string]any{
				"type":        "integer",
				"description": "The maximum number of tokens in the response",
			},
		},
		"required": []string{"image_urls"},
	}

	return tool.NewFunctionTool(
		"openai_image_to_text",
		"Convert image(s) to text using OpenAI vision models. Analyzes images and returns text descriptions.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			imageUrls, _ := input["image_urls"].(string)
			if strings.TrimSpace(imageUrls) == "" {
				return tool.NewErrorResponse(errors.New("image_urls is required")), nil
			}
			prompt := defaultString(input["prompt"], "Describe the image(s) in detail.")
			modelName := defaultString(input["model"], "gpt-4o")
			maxTokens := defaultInt(input["max_tokens"], 300)

			chatModel, err := m.visionChatModel(modelName)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("failed to prepare chat model: %w", err)), nil
			}

			msgBuilder := message.NewMsg().Role(message.RoleUser).TextContent(prompt)
			for _, raw := range strings.Split(imageUrls, ",") {
				url := strings.TrimSpace(raw)
				if url != "" {
					msgBuilder.Content(message.NewImageBlock(url, "", "image/png"))
				}
			}
			msgs := []*message.Msg{msgBuilder.Build()}

			resp, err := chatModel.Chat(ctx, msgs, model.WithMaxTokens(maxTokens))
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("openai vision chat failed: %w", err)), nil
			}

			text := resp.GetTextContent()
			if strings.TrimSpace(text) == "" {
				return tool.NewErrorResponse(errors.New("no content returned from vision model")), nil
			}

			return &tool.Response{
				Content: []message.ContentBlock{message.NewTextBlock(text)},
				IsLast:  true,
			}, nil
		},
	)
}

// AllTools returns every OpenAI multimodal tool in a slice.
func (m *OpenAIMultiModalTool) AllTools() []tool.Tool {
	return []tool.Tool{
		m.TextToImageTool(),
		m.ImageToTextTool(),
	}
}

func defaultString(v any, fallback string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func defaultInt(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return fallback
}
