package multimodal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	dashScopeBaseURL           = "https://dashscope.aliyuncs.com"
	dashScopeEndpointText2Img  = "/api/v1/services/aigc/text2image/image-synthesis"
	dashScopeEndpointText2Vid  = "/api/v1/services/aigc/video-generation/video-synthesis"
	dashScopeEndpointTaskQuery = "/api/v1/tasks/"
)

type dashScopeAsyncClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func newDashScopeAsyncClient(apiKey, baseURL string, httpClient *http.Client) *dashScopeAsyncClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = dashScopeBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &dashScopeAsyncClient{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient,
	}
}

func (c *dashScopeAsyncClient) postJSON(ctx context.Context, endpoint string, body map[string]any) (map[string]any, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("dashscope post %s: invalid json: %w", endpoint, err)
	}

	if resp.StatusCode >= 400 {
		code, _ := result["code"].(string)
		msg, _ := result["message"].(string)
		return nil, fmt.Errorf("dashscope post %s: status=%d code=%s message=%s", endpoint, resp.StatusCode, code, msg)
	}
	return result, nil
}

func (c *dashScopeAsyncClient) getJSON(ctx context.Context, endpoint string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("dashscope get %s: invalid json: %w", endpoint, err)
	}

	if resp.StatusCode >= 400 {
		code, _ := result["code"].(string)
		msg, _ := result["message"].(string)
		return nil, fmt.Errorf("dashscope get %s: status=%d code=%s message=%s", endpoint, resp.StatusCode, code, msg)
	}
	return result, nil
}

func extractTaskID(resp map[string]any) (string, error) {
	output, ok := resp["output"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("missing output in dashscope response")
	}
	taskID, _ := output["task_id"].(string)
	if taskID == "" {
		return "", fmt.Errorf("missing task_id in dashscope response")
	}
	return taskID, nil
}

func pollDashScopeTask(ctx context.Context, client *dashScopeAsyncClient, taskID string, interval, timeout time.Duration) (map[string]any, error) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	endpoint := dashScopeEndpointTaskQuery + taskID
	for {
		resp, err := client.getJSON(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		output, ok := resp["output"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("missing output in task query response")
		}
		status, _ := output["task_status"].(string)
		switch status {
		case "SUCCEEDED":
			return resp, nil
		case "FAILED":
			code, _ := output["code"].(string)
			msg, _ := output["message"].(string)
			return nil, fmt.Errorf("dashscope task failed: code=%s message=%s", code, msg)
		case "CANCELED":
			return nil, fmt.Errorf("dashscope task canceled")
		case "UNKNOWN":
			return nil, fmt.Errorf("dashscope task unknown")
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, fmt.Errorf("dashscope task polling timeout")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}
