package runcontext

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
)

type modelKey struct{}

// WithModel attaches a per-request model override. Middleware (e.g. a model
// router) uses it to route one turn to a cheaper or more capable model
// without mutating the agent, so concurrent turns stay isolated.
func WithModel(ctx context.Context, m model.ChatModel) context.Context {
	if m == nil {
		return ctx
	}
	return context.WithValue(ctx, modelKey{}, m)
}

// Model returns the per-request model override, or nil when absent.
func Model(ctx context.Context) model.ChatModel {
	if v, ok := ctx.Value(modelKey{}).(model.ChatModel); ok {
		return v
	}
	return nil
}
