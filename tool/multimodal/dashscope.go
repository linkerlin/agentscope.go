package multimodal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	ds "github.com/linkerlin/agentscope.go/model/dashscope"
	"github.com/linkerlin/agentscope.go/tool"
)

// DashScopeMultiModalTool wraps DashScope multimodal APIs as tools.
type DashScopeMultiModalTool struct {
	apiKey       string
	baseURL      string
	http         *dashScopeAsyncClient
	chatModel    model.ChatModel
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// NewDashScopeMultiModalTool creates a tool with the given DashScope API key.
func NewDashScopeMultiModalTool(apiKey string) (*DashScopeMultiModalTool, error) {
	return NewDashScopeMultiModalToolWithBaseURL(apiKey, "")
}

// NewDashScopeMultiModalToolWithBaseURL creates a tool with a custom base URL.
func NewDashScopeMultiModalToolWithBaseURL(apiKey, baseURL string) (*DashScopeMultiModalTool, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("dashscope: API key is required")
	}

	db := ds.Builder().APIKey(apiKey)
	if baseURL != "" {
		db.BaseURL(baseURL)
	}
	chatModel, err := db.Build()
	if err != nil {
		return nil, fmt.Errorf("dashscope multimodal: failed to build chat model: %w", err)
	}

	httpClient := newDashScopeAsyncClient(apiKey, baseURL, nil)
	return &DashScopeMultiModalTool{
		apiKey:    apiKey,
		baseURL:   baseURL,
		http:      httpClient,
		chatModel: chatModel,
	}, nil
}

// NewDashScopeMultiModalToolWithClient creates a tool with injected dependencies (for testing).
func NewDashScopeMultiModalToolWithClient(apiKey, baseURL string, httpClient *http.Client, chatModel model.ChatModel) *DashScopeMultiModalTool {
	asyncClient := newDashScopeAsyncClient(apiKey, baseURL, httpClient)
	return &DashScopeMultiModalTool{
		apiKey:    apiKey,
		baseURL:   baseURL,
		http:      asyncClient,
		chatModel: chatModel,
	}
}

func (m *DashScopeMultiModalTool) pollIntervalOr(def time.Duration) time.Duration {
	if m.pollInterval > 0 {
		return m.pollInterval
	}
	return def
}

func (m *DashScopeMultiModalTool) pollTimeoutOr(def time.Duration) time.Duration {
	if m.pollTimeout > 0 {
		return m.pollTimeout
	}
	return def
}

func (m *DashScopeMultiModalTool) visionChatModel(modelName string) (model.ChatModel, error) {
	if modelName == "" {
		return m.chatModel, nil
	}
	if m.chatModel != nil && m.chatModel.ModelName() == modelName {
		return m.chatModel, nil
	}
	db := ds.Builder().APIKey(m.apiKey).ModelName(modelName)
	if m.baseURL != "" {
		db.BaseURL(m.baseURL)
	}
	return db.Build()
}

// TextToImageTool returns a tool that generates images via DashScope.
func (m *DashScopeMultiModalTool) TextToImageTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text prompt to generate image",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "The model to use, e.g., 'wanx-v1', 'qwen-image', 'wan2.2-t2i-flash', etc.",
			},
			"n": map[string]any{
				"type":        "integer",
				"description": "The number of images to generate",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "Size of the image, e.g., '1024*1024', '1280*1280', '800*1200', etc.",
			},
			"use_base64": map[string]any{
				"type":        "boolean",
				"description": "Whether to use base64 data for images",
			},
		},
		"required": []string{"prompt"},
	}

	return tool.NewFunctionTool(
		"dashscope_text_to_image",
		"Generate image(s) based on the given prompt, and return image url(s) or base64 data.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			prompt, _ := input["prompt"].(string)
			if strings.TrimSpace(prompt) == "" {
				return tool.NewErrorResponse(errors.New("prompt is required")), nil
			}

			modelName := defaultString(input["model"], "wanx-v1")
			n := defaultInt(input["n"], 1)
			size := defaultString(input["size"], "1024*1024")
			useBase64 := defaultBool(input["use_base64"], false)

			body := map[string]any{
				"model": modelName,
				"input": map[string]any{
					"prompt": prompt,
				},
				"parameters": map[string]any{
					"n":    n,
					"size": size,
				},
			}

			submitResp, err := m.http.postJSON(ctx, dashScopeEndpointText2Img, body)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope text2image submit failed: %w", err)), nil
			}

			taskID, err := extractTaskID(submitResp)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope text2image submit failed: %w", err)), nil
			}

			resultResp, err := pollDashScopeTask(ctx, m.http, taskID, m.pollIntervalOr(5*time.Second), m.pollTimeoutOr(10*time.Minute))
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope text2image polling failed: %w", err)), nil
			}

			output, _ := resultResp["output"].(map[string]any)
			resultsRaw, _ := output["results"].([]any)
			var blocks []message.ContentBlock
			for _, r := range resultsRaw {
				item, _ := r.(map[string]any)
				url, _ := item["url"].(string)
				if url == "" {
					continue
				}
				if useBase64 {
					// Best-effort download and encode to base64
					b64, mime, err := downloadURLToBase64(ctx, m.http.client, url)
					if err != nil {
						return tool.NewErrorResponse(fmt.Errorf("failed to download image: %w", err)), nil
					}
					blocks = append(blocks, message.NewDataBlock(message.TypeImage, &message.Source{
						Type:      message.SourceTypeBase64,
						MediaType: mime,
						Data:      b64,
					}))
				} else {
					blocks = append(blocks, message.NewDataBlock(message.TypeImage, &message.Source{
						Type: message.SourceTypeURL,
						URL:  url,
					}))
				}
			}

			if len(blocks) == 0 {
				return tool.NewErrorResponse(errors.New("no image url returned")), nil
			}
			return &tool.Response{Content: blocks, IsLast: true}, nil
		},
	)
}

// ImageToTextTool returns a vision tool that converts images to text via DashScope VL models.
func (m *DashScopeMultiModalTool) ImageToTextTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image_urls": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "The URL(s), local file path(s) or Base64 data URL(s) of image(s) to be converted into text.",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text prompt.",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "The model to use, e.g., 'qwen3-vl-plus', 'qwen-vl-plus', 'qwen-vl-max', etc.",
			},
		},
		"required": []string{"image_urls"},
	}

	return tool.NewFunctionTool(
		"dashscope_image_to_text",
		"Generate text based on the given images.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			imageUrlsAny, _ := input["image_urls"].([]any)
			var imageUrls []string
			for _, v := range imageUrlsAny {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					imageUrls = append(imageUrls, s)
				}
			}
			if len(imageUrls) == 0 {
				return tool.NewErrorResponse(errors.New("image_urls is required")), nil
			}

			prompt := defaultString(input["prompt"], "Describe the image")
			modelName := defaultString(input["model"], "qwen3-vl-plus")

			chatModel, err := m.visionChatModel(modelName)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("failed to prepare chat model: %w", err)), nil
			}

			msgBuilder := message.NewMsg().Role(message.RoleUser).TextContent(prompt)
			for _, url := range imageUrls {
				msgBuilder.Content(message.NewImageBlock(url, "", "image/png"))
			}
			msgs := []*message.Msg{msgBuilder.Build()}

			resp, err := chatModel.Chat(ctx, msgs)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope vision chat failed: %w", err)), nil
			}

			text := resp.GetTextContent()
			if strings.TrimSpace(text) == "" {
				return tool.NewErrorResponse(errors.New("no content returned from vision model")), nil
			}
			return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock(text)}, IsLast: true}, nil
		},
	)
}

// TextToVideoTool returns a tool that generates videos from text via DashScope.
func (m *DashScopeMultiModalTool) TextToVideoTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text prompt to generate video",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "The model to use, e.g., 'wan2.6-t2v', 'wan2.5-t2v-preview', etc",
			},
			"negative_prompt": map[string]any{
				"type":        "string",
				"description": "The negative prompt to avoid certain elements",
			},
			"audio_url": map[string]any{
				"type":        "string",
				"description": "The URL for background audio",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "Size of the video, e.g., '1920*1080', '1280*720', etc",
			},
			"duration": map[string]any{
				"type":        "integer",
				"description": "Duration of the video in seconds, e.g., 5, 10",
			},
			"shot_type": map[string]any{
				"type":        "string",
				"description": "Specify the shot type: single (default) or multi",
			},
			"prompt_extend": map[string]any{
				"type":        "boolean",
				"description": "Whether to automatically extend the prompt (default true)",
			},
			"watermark": map[string]any{
				"type":        "boolean",
				"description": "Whether to include watermark (default false)",
			},
			"seed": map[string]any{
				"type":        "integer",
				"description": "The seed for reproducibility",
			},
		},
		"required": []string{"prompt"},
	}

	return tool.NewFunctionTool(
		"dashscope_text_to_video",
		"Generate video based on the given text prompt",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			prompt, _ := input["prompt"].(string)
			if strings.TrimSpace(prompt) == "" {
				return tool.NewErrorResponse(errors.New("prompt is required")), nil
			}

			modelName := defaultString(input["model"], "wan2.6-t2v")
			size := defaultString(input["size"], "1920*1080")
			shotType := defaultString(input["shot_type"], "single")
			promptExtend := defaultBool(input["prompt_extend"], true)
			watermark := defaultBool(input["watermark"], false)

			parameters := map[string]any{
				"size":          size,
				"shot_type":     shotType,
				"prompt_extend": promptExtend,
				"watermark":     watermark,
			}
			if v, ok := input["duration"]; ok && v != nil {
				parameters["duration"] = defaultInt(v, 0)
			}
			if v, ok := input["seed"]; ok && v != nil {
				parameters["seed"] = defaultInt(v, 0)
			}

			bodyInput := map[string]any{
				"prompt": prompt,
			}
			if v, ok := input["negative_prompt"].(string); ok && v != "" {
				bodyInput["negative_prompt"] = v
			}
			if v, ok := input["audio_url"].(string); ok && v != "" {
				bodyInput["audio_url"] = v
			}

			body := map[string]any{
				"model":      modelName,
				"input":      bodyInput,
				"parameters": parameters,
			}

			submitResp, err := m.http.postJSON(ctx, dashScopeEndpointText2Vid, body)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope text2video submit failed: %w", err)), nil
			}

			taskID, err := extractTaskID(submitResp)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope text2video submit failed: %w", err)), nil
			}

			resultResp, err := pollDashScopeTask(ctx, m.http, taskID, m.pollIntervalOr(10*time.Second), m.pollTimeoutOr(30*time.Minute))
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope text2video polling failed: %w", err)), nil
			}

			output, _ := resultResp["output"].(map[string]any)
			videoURL, _ := output["video_url"].(string)
			if videoURL == "" {
				return tool.NewErrorResponse(errors.New("no video url returned")), nil
			}

			return &tool.Response{
				Content: []message.ContentBlock{message.NewVideoBlock(videoURL)},
				IsLast:  true,
			}, nil
		},
	)
}

// ImageToVideoTool returns a tool that generates videos from an image via DashScope.
func (m *DashScopeMultiModalTool) ImageToVideoTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "Text prompt describing the video content and motion",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model to use, e.g., 'wan2.6-i2v-flash', 'wan2.6-i2v', etc",
			},
			"image_url": map[string]any{
				"type":        "string",
				"description": "URL, local file path or Base64 data URL of the first frame image",
			},
			"audio_url": map[string]any{
				"type":        "string",
				"description": "URL of the audio file that the model will use to generate video",
			},
			"negative_prompt": map[string]any{
				"type":        "string",
				"description": "The negative prompt to avoid certain elements",
			},
			"template": map[string]any{
				"type":        "string",
				"description": "Name of video effect template, e.g., 'squish', 'rotation', etc",
			},
			"resolution": map[string]any{
				"type":        "string",
				"description": "Video resolution, e.g., '720P', '1080P'",
			},
			"duration": map[string]any{
				"type":        "integer",
				"description": "Duration of the video in seconds",
			},
			"shot_type": map[string]any{
				"type":        "string",
				"description": "single (default) or multi",
			},
			"audio": map[string]any{
				"type":        "boolean",
				"description": "Whether to generate audio video (default true)",
			},
			"prompt_extend": map[string]any{
				"type":        "boolean",
				"description": "Whether to automatically extend the prompt (default true)",
			},
			"watermark": map[string]any{
				"type":        "boolean",
				"description": "Whether to include watermark (default false)",
			},
			"seed": map[string]any{
				"type":        "integer",
				"description": "Optional seed for reproducibility",
			},
		},
		"required": []string{"image_url"},
	}

	return tool.NewFunctionTool(
		"dashscope_image_to_video",
		"Generate a video from a single input image and an optional text prompt.",
		params,
		func(ctx context.Context, input map[string]any) (*tool.Response, error) {
			imageURL, _ := input["image_url"].(string)
			if strings.TrimSpace(imageURL) == "" {
				return tool.NewErrorResponse(errors.New("image_url is required")), nil
			}

			modelName := defaultString(input["model"], "wan2.6-i2v-flash")
			shotType := defaultString(input["shot_type"], "single")
			audio := defaultBool(input["audio"], true)
			promptExtend := defaultBool(input["prompt_extend"], true)
			watermark := defaultBool(input["watermark"], false)

			parameters := map[string]any{
				"shot_type":     shotType,
				"audio":         audio,
				"prompt_extend": promptExtend,
				"watermark":     watermark,
			}
			if v, ok := input["duration"]; ok && v != nil {
				parameters["duration"] = defaultInt(v, 0)
			}
			if v, ok := input["seed"]; ok && v != nil {
				parameters["seed"] = defaultInt(v, 0)
			}
			if v, ok := input["template"].(string); ok && v != "" {
				parameters["template"] = v
			}
			if v, ok := input["resolution"].(string); ok && v != "" {
				parameters["resolution"] = v
			}

			bodyInput := map[string]any{
				"image_url": imageURL,
			}
			if v, ok := input["prompt"].(string); ok && v != "" {
				bodyInput["prompt"] = v
			}
			if v, ok := input["negative_prompt"].(string); ok && v != "" {
				bodyInput["negative_prompt"] = v
			}
			if v, ok := input["audio_url"].(string); ok && v != "" {
				bodyInput["audio_url"] = v
			}

			body := map[string]any{
				"model":      modelName,
				"input":      bodyInput,
				"parameters": parameters,
			}

			submitResp, err := m.http.postJSON(ctx, dashScopeEndpointText2Vid, body)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope image2video submit failed: %w", err)), nil
			}

			taskID, err := extractTaskID(submitResp)
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope image2video submit failed: %w", err)), nil
			}

			resultResp, err := pollDashScopeTask(ctx, m.http, taskID, m.pollIntervalOr(10*time.Second), m.pollTimeoutOr(30*time.Minute))
			if err != nil {
				return tool.NewErrorResponse(fmt.Errorf("dashscope image2video polling failed: %w", err)), nil
			}

			output, _ := resultResp["output"].(map[string]any)
			videoURL, _ := output["video_url"].(string)
			if videoURL == "" {
				return tool.NewErrorResponse(errors.New("no video url returned")), nil
			}

			return &tool.Response{
				Content: []message.ContentBlock{message.NewVideoBlock(videoURL)},
				IsLast:  true,
			}, nil
		},
	)
}

// AllTools returns every DashScope multimodal tool in a slice.
func (m *DashScopeMultiModalTool) AllTools() []tool.Tool {
	return []tool.Tool{
		m.TextToImageTool(),
		m.ImageToTextTool(),
		m.TextToVideoTool(),
		m.ImageToVideoTool(),
	}
}

func defaultBool(v any, fallback bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}
