package mcp

import (
	"context"
	"testing"
)

type mockElicitationClient struct {
	mockClient
	req  ElicitRequest
	resp ElicitResult
	err  error
}

func (m *mockElicitationClient) Elicit(ctx context.Context, req ElicitRequest) (ElicitResult, error) {
	m.req = req
	return m.resp, m.err
}

func TestElicit_Supported(t *testing.T) {
	ctx := context.Background()
	mock := &mockElicitationClient{resp: ElicitResult{Accepted: true, Data: map[string]any{"value": 42}}}

	req := ElicitRequest{Message: "confirm?", Data: map[string]any{"id": 1}}
	resp, err := Elicit(ctx, mock, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Accepted {
		t.Fatal("expected accepted result")
	}
	if mock.req.Message != req.Message {
		t.Fatalf("expected request message %q, got %q", req.Message, mock.req.Message)
	}
}

func TestElicit_Unsupported(t *testing.T) {
	ctx := context.Background()
	mock := &mockClient{}

	_, err := Elicit(ctx, mock, ElicitRequest{Message: "confirm?"})
	if err == nil {
		t.Fatal("expected error for unsupported client")
	}
}

func TestElicitationClient_Interface(t *testing.T) {
	var _ ElicitationClient = (*mockElicitationClient)(nil)
}
