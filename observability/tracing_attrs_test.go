package observability

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/tool"
)

type trStubAgent struct{ name string }

func (s *trStubAgent) AgentName() string { return s.name }

func TestTracingAdapter_RichAttributes(t *testing.T) {
	rt := &RecordingTracer{}
	adapter := &TracingMiddlewareAdapter{Tracer: rt, Name: "agentA"}
	ctx := context.Background()
	agent := &trStubAgent{"agentA"}

	// OnReply: message count + last role
	adapter.OnReply(ctx, agent, &middleware.ReplyInput{
		Messages: []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
			message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(),
		},
	}, func(ctx context.Context) (*message.Msg, error) { return nil, nil })
	if sp := rt.SpanByName("agentA_on_reply"); sp == nil {
		t.Fatal("on_reply span missing")
	} else {
		if sp.Attr("reply.message_count") != 2 {
			t.Fatalf("message_count attr = %v", sp.Attr("reply.message_count"))
		}
		if sp.Attr("reply.last_role") != string(message.RoleAssistant) {
			t.Fatalf("last_role attr = %v", sp.Attr("reply.last_role"))
		}
	}

	// OnReasoning: iteration
	adapter.OnReasoning(ctx, agent, &middleware.ReasoningInput{Iteration: 3}, func(ctx context.Context) (*message.Msg, error) {
		return nil, nil
	})
	if sp := rt.SpanByName("agentA_on_reasoning"); sp == nil {
		t.Fatal("on_reasoning span missing")
	} else if sp.Attr("reasoning.iteration") != 3 {
		t.Fatalf("iteration attr = %v", sp.Attr("reasoning.iteration"))
	}

	// OnActing: tool name + input keys
	adapter.OnActing(ctx, agent, &middleware.ActingInput{ToolName: "bash", ToolInput: map[string]any{"cmd": "ls", "cwd": "/tmp"}}, func(ctx context.Context) (*tool.Response, error) {
		return nil, nil
	})
	if sp := rt.SpanByName("agentA_on_acting"); sp == nil {
		t.Fatal("on_acting span missing")
	} else {
		if sp.Attr("tool.name") != "bash" {
			t.Fatalf("tool.name attr = %v", sp.Attr("tool.name"))
		}
		if sp.Attr("tool.input_keys") != 2 {
			t.Fatalf("input_keys attr = %v", sp.Attr("tool.input_keys"))
		}
	}

	// OnModelCall: model name + message count
	adapter.OnModelCall(ctx, agent, &middleware.ModelCallInput{ModelName: "gpt-4o", Messages: []*message.Msg{{}}}, func(ctx context.Context) (*message.Msg, error) {
		return nil, nil
	})
	if sp := rt.SpanByName("agentA_on_model_call"); sp == nil {
		t.Fatal("on_model_call span missing")
	} else {
		if sp.Attr("model.name") != "gpt-4o" {
			t.Fatalf("model.name attr = %v", sp.Attr("model.name"))
		}
		if sp.Attr("model.message_count") != 1 {
			t.Fatalf("message_count attr = %v", sp.Attr("model.message_count"))
		}
	}

	// OnSystemPrompt: prompt length
	adapter.OnSystemPrompt(ctx, agent, "a short prompt")
	if sp := rt.SpanByName("agentA_on_system_prompt"); sp == nil {
		t.Fatal("on_system_prompt span missing")
	} else if sp.Attr("system_prompt.length") != len("a short prompt") {
		t.Fatalf("prompt.length attr = %v", sp.Attr("system_prompt.length"))
	}
}

func TestRecordingSpan_AttrNotFound(t *testing.T) {
	s := &RecordingSpan{Name: "x"}
	if s.Attr("missing") != nil {
		t.Fatal("missing attr should be nil")
	}
}

func TestRecordingTracer_LastSpan(t *testing.T) {
	rt := &RecordingTracer{}
	rt.Start(context.Background(), "a")
	rt.Start(context.Background(), "b")
	if rt.LastSpan().Name != "b" {
		t.Fatal("LastSpan should be b")
	}
	if rt.SpanByName("a") == nil {
		t.Fatal("SpanByName(a) should find it")
	}
	if rt.SpanByName("z") != nil {
		t.Fatal("SpanByName(z) should be nil")
	}
}

func TestNoopSpan_SetAttributes(t *testing.T) {
	var s Span = noopSpan{}
	s.SetAttributes(StringAttr("k", "v")) // must not panic
	s.End()
}

func TestSpanAttrHelpers(t *testing.T) {
	if StringAttr("k", "v").Value != "v" {
		t.Fatal()
	}
	if IntAttr("k", 5).Value != 5 {
		t.Fatal()
	}
	if BoolAttr("k", true).Value != true {
		t.Fatal()
	}
	if Float64Attr("k", 1.5).Value != 1.5 {
		t.Fatal()
	}
}
