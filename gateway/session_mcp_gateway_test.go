package gateway

import (
	"context"
	"testing"
)

func TestSessionMCPGatewayPool_DifferentSessions(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionMCPGatewayPool("per-session-token")

	u1, tok1, err := pool.Ensure(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	u2, tok2, err := pool.Ensure(ctx, "s2")
	if err != nil {
		t.Fatal(err)
	}
	if u1 == u2 {
		t.Fatalf("expected different gateway URLs, both %s", u1)
	}
	if tok1 != tok2 || tok1 != "per-session-token" {
		t.Fatalf("unexpected token: %q %q", tok1, tok2)
	}

	u1Again, _, err := pool.Ensure(ctx, "s1")
	if err != nil || u1Again != u1 {
		t.Fatalf("expected stable URL for s1: %s vs %s", u1, u1Again)
	}

	if err := pool.Close(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := pool.CloseAll(ctx); err != nil {
		t.Fatal(err)
	}
}
