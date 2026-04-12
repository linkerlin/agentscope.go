package a2a

import (
	"context"
	"testing"
)

func TestNoopClient(t *testing.T) {
	var c NoopClient
	_, err := c.Send(context.Background(), &Message{Role: "user", Content: "hi"})
	if err == nil || err.Error() != "a2a: noop client" {
		t.Fatalf("got %v", err)
	}
	if c.Close() != nil {
		t.Fatal()
	}
}
