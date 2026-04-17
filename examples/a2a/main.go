package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// mockModel is a simple ChatModel that returns a fixed text response.
// It allows the example to run without external API keys.
type mockModel struct {
	response string
}

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().
		Role(message.RoleAssistant).
		TextContent(m.response).
		Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 1)
	go func() {
		ch <- &model.StreamChunk{Delta: m.response, Done: true}
	}()
	return ch, nil
}

func (m *mockModel) ModelName() string { return "mock-model" }

// sendTask manually POSTs to /task/send with a known task ID so that we can
// poll for completion afterwards. a2a.HTTPClient.Send creates a task internally
// but does not expose the generated task ID, so we track our own for WaitForTask.
func sendTask(baseURL string, msg *a2a.Message) (string, *a2a.Message, error) {
	taskID := a2a.NewTaskID()
	reqBody, _ := json.Marshal(a2a.TaskUpdateRequest{ID: taskID, Message: msg})
	resp, err := http.Post(baseURL+"/task/send", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return "", nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var task a2a.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return "", nil, err
	}
	if len(task.Messages) == 0 {
		return "", nil, fmt.Errorf("no messages in task")
	}
	last := task.Messages[len(task.Messages)-1]
	return taskID, &last, nil
}

func main() {
	// 1. Create a ReActAgent backed by a mock model.
	agent, err := react.Builder().
		Name("A2AAgent").
		SysPrompt("You are a helpful assistant.").
		Model(&mockModel{response: "Hello from A2A!"}).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Wrap the agent with the A2A adapter so it satisfies AgentRunner.
	adapter := a2a.NewAgentAdapter(agent)

	// 3. Create and start the A2A HTTP server on a random free port.
	card := a2a.AgentCard{
		Name:    "A2AAgent",
		URL:     "http://localhost:0",
		Version: "1.0.0",
	}
	store := a2a.NewInMemoryTaskStore()
	server := a2a.NewServer(card, adapter, store)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	baseURL := "http://" + listener.Addr().String()

	go func() {
		if err := http.Serve(listener, server); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	// Give the server a moment to start listening.
	time.Sleep(100 * time.Millisecond)

	// 4. Use a2a.HTTPClient to send a message.
	client := a2a.NewHTTPClient(baseURL)
	msg := &a2a.Message{Role: "user", Content: "Hi there!"}

	// Send posts the message and returns the last message in the initial task snapshot.
	reply, err := client.Send(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Send reply (initial snapshot): %s\n", reply.Content)

	// 5. Create another task with a known ID and wait for it to reach a terminal status.
	//    The mock model replies instantly, so the task will complete quickly.
	taskID, _, err := sendTask(baseURL, msg)
	if err != nil {
		log.Fatal(err)
	}

	task, err := client.WaitForTask(context.Background(), taskID, 50*time.Millisecond)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Task %s finished with status: %s\n", task.ID, task.Status)
	if len(task.Messages) > 0 {
		last := task.Messages[len(task.Messages)-1]
		fmt.Printf("Final message: %s\n", last.Content)
	}
}
