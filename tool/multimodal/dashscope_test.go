package multimodal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

func TestDashScopeMultiModalTool_TextToImage(t *testing.T) {
	var queries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/services/aigc/text2image/image-synthesis" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":    "task-123",
					"task_status": "PENDING",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/tasks/") {
			q := queries.Add(1)
			status := "PENDING"
			if q >= 2 {
				status = "SUCCEEDED"
			}
			result := map[string]any{
				"output": map[string]any{
					"task_id":     "task-123",
					"task_status": status,
				},
			}
			if status == "SUCCEEDED" {
				output := result["output"].(map[string]any)
				output["results"] = []map[string]any{
					{"url": "https://dashscope.example.com/img.png"},
				}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewDashScopeMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	mmt.pollInterval = 50 * time.Millisecond
	mmt.pollTimeout = 1 * time.Second
	toolFn := mmt.TextToImageTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
		"n":      1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(resp.Content))
	}
	data, ok := resp.Content[0].(*message.DataBlock)
	if !ok {
		t.Fatalf("expected DataBlock, got %T", resp.Content[0])
	}
	if data.Source == nil || data.Source.URL != "https://dashscope.example.com/img.png" {
		t.Fatalf("unexpected source: %+v", data.Source)
	}
}

func TestDashScopeMultiModalTool_TextToImage_Base64(t *testing.T) {
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fakepng"))
	}))
	defer imgServer.Close()

	var queries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/services/aigc/text2image/image-synthesis" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":    "task-123",
					"task_status": "PENDING",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/tasks/") {
			q := queries.Add(1)
			status := "PENDING"
			if q >= 2 {
				status = "SUCCEEDED"
			}
			result := map[string]any{
				"output": map[string]any{
					"task_id":     "task-123",
					"task_status": status,
				},
			}
			if status == "SUCCEEDED" {
				output := result["output"].(map[string]any)
				output["results"] = []map[string]any{
					{"url": imgServer.URL + "/img.png"},
				}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewDashScopeMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	mmt.pollInterval = 50 * time.Millisecond
	mmt.pollTimeout = 1 * time.Second
	toolFn := mmt.TextToImageTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"prompt":     "a cat",
		"use_base64": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resp.Content[0].(*message.DataBlock)
	if !ok {
		t.Fatalf("expected DataBlock, got %T", resp.Content[0])
	}
	if data.Source == nil || data.Source.Type != message.SourceTypeBase64 {
		t.Fatalf("expected base64 source")
	}
	if data.Source.Data == "" {
		t.Fatal("expected non-empty base64 data")
	}
}

func TestDashScopeMultiModalTool_ImageToText(t *testing.T) {
	mock := &mockChatModel{modelName: "qwen3-vl-plus", respText: "一只猫。"}
	mmt := NewDashScopeMultiModalToolWithClient("sk-test", "", nil, mock)
	toolFn := mmt.ImageToTextTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"image_urls": []any{"https://example.com/cat.png"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "一只猫。" {
		t.Fatalf("unexpected text: %s", resp.GetTextContent())
	}
}

func TestDashScopeMultiModalTool_TextToVideo(t *testing.T) {
	var queries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":    "task-vid",
					"task_status": "PENDING",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/tasks/") {
			q := queries.Add(1)
			status := "PENDING"
			if q >= 2 {
				status = "SUCCEEDED"
			}
			result := map[string]any{
				"output": map[string]any{
					"task_id":     "task-vid",
					"task_status": status,
				},
			}
			if status == "SUCCEEDED" {
				result["output"].(map[string]any)["video_url"] = "https://dashscope.example.com/vid.mp4"
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewDashScopeMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	mmt.pollInterval = 50 * time.Millisecond
	mmt.pollTimeout = 1 * time.Second
	toolFn := mmt.TextToVideoTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"prompt": "a cat running",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(resp.Content))
	}
	vid, ok := resp.Content[0].(*message.VideoBlock)
	if !ok {
		t.Fatalf("expected VideoBlock, got %T", resp.Content[0])
	}
	if vid.URL != "https://dashscope.example.com/vid.mp4" {
		t.Fatalf("unexpected url: %s", vid.URL)
	}
}

func TestDashScopeMultiModalTool_ImageToVideo(t *testing.T) {
	var queries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":    "task-i2v",
					"task_status": "PENDING",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/tasks/") {
			q := queries.Add(1)
			status := "PENDING"
			if q >= 2 {
				status = "SUCCEEDED"
			}
			result := map[string]any{
				"output": map[string]any{
					"task_id":     "task-i2v",
					"task_status": status,
				},
			}
			if status == "SUCCEEDED" {
				result["output"].(map[string]any)["video_url"] = "https://dashscope.example.com/i2v.mp4"
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewDashScopeMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	mmt.pollInterval = 50 * time.Millisecond
	mmt.pollTimeout = 1 * time.Second
	toolFn := mmt.ImageToVideoTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"image_url": "https://example.com/frame.png",
		"prompt":    "make it move",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vid, ok := resp.Content[0].(*message.VideoBlock)
	if !ok {
		t.Fatalf("expected VideoBlock, got %T", resp.Content[0])
	}
	if vid.URL != "https://dashscope.example.com/i2v.mp4" {
		t.Fatalf("unexpected url: %s", vid.URL)
	}
}

func TestDashScopeMultiModalTool_TextToImage_TaskFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/services/aigc/text2image/image-synthesis" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":    "task-fail",
					"task_status": "PENDING",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/tasks/") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":     "task-fail",
					"task_status": "FAILED",
					"code":        "InvalidParameter",
					"message":     "bad size",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewDashScopeMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	mmt.pollInterval = 50 * time.Millisecond
	mmt.pollTimeout = 1 * time.Second
	toolFn := mmt.TextToImageTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error text")
	}
	if !strings.Contains(resp.GetTextContent(), "bad size") {
		t.Fatalf("expected error message containing 'bad size', got %s", resp.GetTextContent())
	}
}

func TestDashScopeMultiModalTool_TextToVideo_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":    "task-to",
					"task_status": "PENDING",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/tasks/") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"output": map[string]any{
					"task_id":     "task-to",
					"task_status": "PENDING",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mmt := NewDashScopeMultiModalToolWithClient("sk-test", server.URL, server.Client(), nil)
	mmt.pollInterval = 50 * time.Millisecond
	mmt.pollTimeout = 1 * time.Second
	toolFn := mmt.TextToVideoTool()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	resp, err := toolFn.Execute(ctx, map[string]any{
		"prompt": "a cat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error text")
	}
}

func TestDashScopeMultiModalTool_NewMissingKey(t *testing.T) {
	_, err := NewDashScopeMultiModalTool("  ")
	if err == nil {
		t.Fatal("expected error for empty api key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDashScopeMultiModalTool_ImageToText_MissingImageURLs(t *testing.T) {
	mock := &mockChatModel{modelName: "qwen3-vl-plus", respText: ""}
	mmt := NewDashScopeMultiModalToolWithClient("sk-test", "", nil, mock)
	toolFn := mmt.ImageToTextTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{
		"image_urls": []any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error text")
	}
}

func TestDashScopeMultiModalTool_ImageToVideo_MissingImageURL(t *testing.T) {
	mmt := NewDashScopeMultiModalToolWithClient("sk-test", "", nil, nil)
	toolFn := mmt.ImageToVideoTool()

	resp, err := toolFn.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("expected error text")
	}
}

func TestDefaultHelpers(t *testing.T) {
	if defaultString(nil, "fallback") != "fallback" {
		t.Fatal("defaultString fallback failed")
	}
	if defaultString("hello", "fallback") != "hello" {
		t.Fatal("defaultString value failed")
	}
	if defaultInt(nil, 5) != 5 {
		t.Fatal("defaultInt fallback failed")
	}
	if defaultInt(float64(3), 5) != 3 {
		t.Fatal("defaultInt float64 failed")
	}
	if defaultBool(nil, true) != true {
		t.Fatal("defaultBool fallback failed")
	}
	if defaultBool(false, true) != false {
		t.Fatal("defaultBool value failed")
	}
}

func TestParseDataURL(t *testing.T) {
	mime, b64, err := parseDataURL("data:image/png;base64,iVBORw0KGgo=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "image/png" {
		t.Fatalf("unexpected mime: %s", mime)
	}
	if b64 != "iVBORw0KGgo=" {
		t.Fatalf("unexpected data: %s", b64)
	}

	_, _, err = parseDataURL("notdata://x")
	if err == nil {
		t.Fatal("expected error")
	}

	_, _, err = parseDataURL("data:missing-comma")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadURLToBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png"))
	}))
	defer server.Close()

	b64, mime, err := downloadURLToBase64(context.Background(), nil, server.URL+"/img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "image/png" {
		t.Fatalf("unexpected mime: %s", mime)
	}
	if b64 == "" {
		t.Fatal("expected non-empty base64")
	}
}

func TestDownloadURLToBase64_DataURL(t *testing.T) {
	b64, mime, err := downloadURLToBase64(context.Background(), nil, "data:image/jpeg;base64,/9j=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "image/jpeg" {
		t.Fatalf("unexpected mime: %s", mime)
	}
	if b64 != "/9j=" {
		t.Fatalf("unexpected data: %s", b64)
	}
}

func TestPollDashScopeTask_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_id":     "t1",
				"task_status": "PENDING",
			},
		})
	}))
	defer server.Close()

	client := newDashScopeAsyncClient("sk", server.URL, server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := pollDashScopeTask(ctx, client, "t1", 100*time.Millisecond, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected context error, got: %v", err)
	}
}

func TestPollDashScopeTask_ImmediateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_id":     "t1",
				"task_status": "SUCCEEDED",
				"video_url":   "https://v.mp4",
			},
		})
	}))
	defer server.Close()

	client := newDashScopeAsyncClient("sk", server.URL, server.Client())
	resp, err := pollDashScopeTask(context.Background(), client, "t1", 100*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := resp["output"].(map[string]any)
	if output["video_url"] != "https://v.mp4" {
		t.Fatal("unexpected result")
	}
}

func TestPollDashScopeTask_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"task_id":     "t1",
				"task_status": "PENDING",
			},
		})
	}))
	defer server.Close()

	client := newDashScopeAsyncClient("sk", server.URL, server.Client())
	_, err := pollDashScopeTask(context.Background(), client, "t1", 50*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestExtractTaskID_Missing(t *testing.T) {
	_, err := extractTaskID(map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = extractTaskID(map[string]any{"output": map[string]any{}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashScopeAsyncClient_PostError(t *testing.T) {
	client := newDashScopeAsyncClient("sk", "http://127.0.0.1:1", nil)
	_, err := client.postJSON(context.Background(), "/test", map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashScopeAsyncClient_GetError(t *testing.T) {
	client := newDashScopeAsyncClient("sk", "http://127.0.0.1:1", nil)
	_, err := client.getJSON(context.Background(), "/test")
	if err == nil {
		t.Fatal("expected error")
	}
}
