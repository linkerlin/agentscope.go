package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// hookStubAgent satisfies the Agent interface.
type hookStubAgent struct{ name string }

func (s *hookStubAgent) AgentName() string { return s.name }

// --- PermissionInterceptor test ---

type auditPermissionMW struct {
	Base
	called  bool
	allowed bool
}

func (m *auditPermissionMW) OnCheckPermission(ctx context.Context, agent Agent, input *PermissionInput, next PermissionNext) (PermissionResult, error) {
	m.called = true
	res, err := next(ctx)
	if err == nil && res.Decision == "allow" {
		m.allowed = true
	}
	return res, err
}

type bypassPermissionMW struct{ Base }

func (m *bypassPermissionMW) OnCheckPermission(ctx context.Context, agent Agent, input *PermissionInput, next PermissionNext) (PermissionResult, error) {
	return PermissionResult{
		ToolCallID: input.ToolCallID,
		ToolName:   input.ToolName,
		Decision:   "allow",
		Message:    "bypassed by middleware",
	}, nil
}

func TestChainPermission_OnionOrder(t *testing.T) {
	agent := &hookStubAgent{"test"}
	audit := &auditPermissionMW{}
	chain := &Chain{Permission: []PermissionInterceptor{audit}}
	input := &PermissionInput{ToolCallID: "tc1", ToolName: "Read", ToolInput: map[string]any{"path": "/tmp"}}

	final := PermissionNext(func(ctx context.Context) (PermissionResult, error) {
		return PermissionResult{ToolCallID: "tc1", ToolName: "Read", Decision: "allow"}, nil
	})

	next := ChainPermission(chain, agent, input, final)
	res, err := next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !audit.called {
		t.Fatal("audit middleware was not called")
	}
	if !audit.allowed {
		t.Fatal("audit middleware did not see allow decision")
	}
	if res.Decision != "allow" {
		t.Fatalf("expected allow, got %s", res.Decision)
	}
}

func TestChainPermission_Bypass(t *testing.T) {
	agent := &hookStubAgent{"test"}
	bypass := &bypassPermissionMW{}
	chain := &Chain{Permission: []PermissionInterceptor{bypass}}
	input := &PermissionInput{ToolCallID: "tc1", ToolName: "Bash", ToolInput: map[string]any{"command": "rm -rf /"}}

	called := false
	final := PermissionNext(func(ctx context.Context) (PermissionResult, error) {
		called = true
		return PermissionResult{ToolCallID: "tc1", Decision: "deny"}, nil
	})

	next := ChainPermission(chain, agent, input, final)
	res, err := next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("final handler should not be called when middleware bypasses")
	}
	if res.Decision != "allow" {
		t.Fatalf("bypass should force allow, got %s", res.Decision)
	}
}

func TestChainPermission_NilChain(t *testing.T) {
	called := false
	final := PermissionNext(func(ctx context.Context) (PermissionResult, error) {
		called = true
		return PermissionResult{Decision: "allow"}, nil
	})
	next := ChainPermission(nil, &hookStubAgent{"x"}, &PermissionInput{}, final)
	res, err := next(context.Background())
	if err != nil || res.Decision != "allow" {
		t.Fatalf("nil chain should pass through, got res=%+v err=%v", res, err)
	}
	if !called {
		t.Fatal("final should be called with nil chain")
	}
}

// --- CompressionInterceptor test ---

type auditCompressionMW struct {
	Base
	called bool
}

func (m *auditCompressionMW) OnCompressContext(ctx context.Context, agent Agent, input *CompressionInput, next CompressionNext) error {
	m.called = true
	return next(ctx)
}

type skipCompressionMW struct{ Base }

func (m *skipCompressionMW) OnCompressContext(ctx context.Context, agent Agent, input *CompressionInput, next CompressionNext) error {
	return nil
}

func TestChainCompression_OnionOrder(t *testing.T) {
	agent := &hookStubAgent{"test"}
	audit := &auditCompressionMW{}
	chain := &Chain{Compression: []CompressionInterceptor{audit}}
	input := &CompressionInput{}

	called := false
	final := CompressionNext(func(ctx context.Context) error {
		called = true
		return nil
	})

	next := ChainCompression(chain, agent, input, final)
	err := next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !audit.called {
		t.Fatal("audit middleware was not called")
	}
	if !called {
		t.Fatal("final handler was not called")
	}
}

func TestChainCompression_Skip(t *testing.T) {
	agent := &hookStubAgent{"test"}
	skip := &skipCompressionMW{}
	chain := &Chain{Compression: []CompressionInterceptor{skip}}
	input := &CompressionInput{}

	called := false
	final := CompressionNext(func(ctx context.Context) error {
		called = true
		return nil
	})

	next := ChainCompression(chain, agent, input, final)
	err := next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("final handler should not be called when middleware skips")
	}
}

func TestChainCompression_Error(t *testing.T) {
	agent := &hookStubAgent{"test"}
	errExpected := errors.New("compression failed")
	chain := &Chain{Compression: []CompressionInterceptor{
		&compressionErrorMW{err: errExpected},
	}}
	input := &CompressionInput{}

	final := CompressionNext(func(ctx context.Context) error {
		t.Fatal("final should not be called on error")
		return nil
	})

	next := ChainCompression(chain, agent, input, final)
	err := next(context.Background())
	if !errors.Is(err, errExpected) {
		t.Fatalf("expected error %v, got %v", errExpected, err)
	}
}

type compressionErrorMW struct {
	Base
	err error
}

func (m *compressionErrorMW) OnCompressContext(ctx context.Context, agent Agent, input *CompressionInput, next CompressionNext) error {
	return m.err
}

// --- Classify test ---

type multiHookMW struct {
	Base
}

func (m *multiHookMW) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	return next(ctx)
}

func (m *multiHookMW) OnCheckPermission(ctx context.Context, agent Agent, input *PermissionInput, next PermissionNext) (PermissionResult, error) {
	return next(ctx)
}

func (m *multiHookMW) OnCompressContext(ctx context.Context, agent Agent, input *CompressionInput, next CompressionNext) error {
	return next(ctx)
}

func TestClassify_NewHooks(t *testing.T) {
	mw := &multiHookMW{}
	chain := Classify([]Middleware{mw})
	if len(chain.Reply) != 1 {
		t.Fatalf("expected 1 Reply interceptor, got %d", len(chain.Reply))
	}
	if len(chain.Permission) != 1 {
		t.Fatalf("expected 1 Permission interceptor, got %d", len(chain.Permission))
	}
	if len(chain.Compression) != 1 {
		t.Fatalf("expected 1 Compression interceptor, got %d", len(chain.Compression))
	}
}
