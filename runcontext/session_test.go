package runcontext

import (
	"context"
	"testing"
)

func TestSessionIDRoundTrip(t *testing.T) {
	ctx := WithSessionID(context.Background(), "sess-1")
	if SessionID(ctx) != "sess-1" {
		t.Fatalf("got %q", SessionID(ctx))
	}
	if SessionID(context.Background()) != "" {
		t.Fatal("expected empty session on bare context")
	}
}
