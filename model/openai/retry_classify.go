package openai

import (
	"context"
	"errors"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/retry"
)

// classifyOpenAIErr 将 OpenAI HTTP 错误分为可重试与永久失败（4xx 非 429 不重试）
func classifyOpenAIErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return retry.Permanent(err)
	}
	var api *goopenai.APIError
	if errors.As(err, &api) && api.HTTPStatusCode > 0 {
		if api.HTTPStatusCode == 429 || api.HTTPStatusCode >= 500 {
			return err
		}
		if api.HTTPStatusCode >= 400 {
			return retry.Permanent(err)
		}
	}
	var req *goopenai.RequestError
	if errors.As(err, &req) && req.HTTPStatusCode > 0 {
		if req.HTTPStatusCode == 429 || req.HTTPStatusCode >= 500 {
			return err
		}
		if req.HTTPStatusCode >= 400 {
			return retry.Permanent(err)
		}
	}
	return err
}
