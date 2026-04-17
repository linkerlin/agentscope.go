package a2a

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient is a concrete HTTP implementation of the Client interface.
type HTTPClient struct {
	baseURL string
	http    *http.Client
}

// NewHTTPClient creates an A2A HTTP client targeting the given base URL.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{},
	}
}

// Send posts a message to the agent's /task/send endpoint.
func (c *HTTPClient) Send(ctx context.Context, msg *Message) (*Message, error) {
	reqBody, err := json.Marshal(TaskUpdateRequest{ID: NewTaskID(), Message: msg})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/task/send", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("a2a send: %s %s", resp.Status, string(body))
	}

	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("a2a send: %w", err)
	}
	if len(task.Messages) == 0 {
		return nil, fmt.Errorf("a2a send: no messages in task response")
	}
	// Return the last message (agent reply or error).
	last := task.Messages[len(task.Messages)-1]
	return &last, nil
}

// SendSubscribe opens an SSE stream to the agent's /task/sendSubscribe endpoint.
// The returned channel yields each streamed message chunk.
func (c *HTTPClient) SendSubscribe(ctx context.Context, msg *Message) (<-chan *Message, error) {
	reqBody, err := json.Marshal(TaskUpdateRequest{ID: NewTaskID(), Message: msg})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/task/sendSubscribe", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a subscribe: %w", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("a2a subscribe: %s %s", resp.Status, string(body))
	}

	ch := make(chan *Message, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}
			var task Task
			if err := json.Unmarshal([]byte(data), &task); err != nil {
				continue
			}
			if len(task.Messages) > 0 {
				last := task.Messages[len(task.Messages)-1]
				ch <- &last
			}
		}
	}()
	return ch, nil
}

// WaitForTask polls GET /task/{id} until the task reaches a terminal status.
func (c *HTTPClient) WaitForTask(ctx context.Context, taskID string, pollInterval time.Duration) (*Task, error) {
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/task/"+taskID, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("a2a wait: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("a2a wait: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("a2a wait: %s %s", resp.Status, string(body))
		}
		var task Task
		if err := json.Unmarshal(body, &task); err != nil {
			return nil, fmt.Errorf("a2a wait: %w", err)
		}
		switch task.Status {
		case TaskStatusCompleted, TaskStatusFailed, TaskStatusCanceled:
			return &task, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// CancelTask posts to /task/cancel to cancel a task.
func (c *HTTPClient) CancelTask(ctx context.Context, taskID string) (*Task, error) {
	reqBody, err := json.Marshal(map[string]string{"id": taskID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/task/cancel", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a cancel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("a2a cancel: %s %s", resp.Status, string(body))
	}

	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("a2a cancel: %w", err)
	}
	return &task, nil
}

// Close is a no-op for the HTTP client.
func (c *HTTPClient) Close() error { return nil }

var _ Client = (*HTTPClient)(nil)
