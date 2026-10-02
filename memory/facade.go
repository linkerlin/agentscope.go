// memory/facade.go — the single assembly entry for the memory stack (20.8):
// the four memory tiers (window, ReMe, agentic, long-term) build from ONE
// place, so examples and applications no longer wire internal constructors
// directly. Every tier is optional (nil options = tier off).
//
// Tiers:
//   - Window:  the in-conversation agent memory (memory.Memory);
//   - ReMe:    cross-session retrieval memory (file-backed, or vector-backed
//     when Store+Embed are provided) plus its hook;
//   - Agentic: agent-managed markdown memory (middleware);
//   - LongTerm: mem0-style long-term memory (middleware) bridged over the
//     facade's own vector ReMe tier — it requires ReMe with a vector store.
package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/middleware"
)

// FacadeOptions configures the memory tiers; every tier is optional.
type FacadeOptions struct {
	Window   *WindowOptions
	ReMe     *ReMeOptions
	Agentic  *AgenticOptions
	LongTerm *LongTermOptions
}

// ReMeOptions enables cross-session retrieval memory. Vector-backed when
// Store+Embed are BOTH set; file-backed otherwise.
type ReMeOptions struct {
	// WorkingDir is the ReMe working directory (dialog + memory files).
	WorkingDir string
	// Vector tier (optional): a vector store plus its embedding model.
	Store VectorStore
	Embed EmbeddingModel
	// Counter is the token estimator (nil = SimpleTokenCounter).
	Counter TokenCounter
}

// AgenticOptions enables agent-managed markdown memory.
type AgenticOptions struct {
	// Dir is the memory directory (layout ensured on build).
	Dir string
}

// LongTermOptions enables mem0-style long-term memory bridged over the
// facade's vector ReMe tier (requires ReMeOptions with Store+Embed).
type LongTermOptions struct {
	// UserID namespaces the memories (per-user isolation).
	UserID string
	// TopK caps retrieval results (default 5).
	TopK int
}

// Facade is the assembled memory stack.
type Facade struct {
	window   Memory
	reme     ReMeMemory
	remeHook hook.Hook
	mws      []middleware.Middleware
}

// NewFacade assembles the configured tiers. The result exposes the agent
// wiring points: Window (agent memory), Hooks (agent hooks: ReMe) and
// Middlewares (agent chain: Agentic + LongTerm).
func NewFacade(opts FacadeOptions) (*Facade, error) {
	f := &Facade{}

	if opts.Window != nil {
		f.window = NewWindowMemory(*opts.Window)
	}

	var vectorReMe *ReMeVectorMemory
	if r := opts.ReMe; r != nil {
		cfg := DefaultReMeFileConfig()
		if r.WorkingDir != "" {
			cfg.WorkingDir = r.WorkingDir
		}
		counter := r.Counter
		if counter == nil {
			counter = NewSimpleTokenCounter()
		}
		switch {
		case r.Store != nil && r.Embed != nil:
			mem, err := NewReMeVectorMemory(cfg, counter, r.Store, r.Embed)
			if err != nil {
				return nil, fmt.Errorf("memory facade: reme vector: %w", err)
			}
			vectorReMe = mem
			f.reme = mem
		case r.Store != nil || r.Embed != nil:
			return nil, errors.New("memory facade: reme vector tier needs BOTH Store and Embed (or neither for file-backed)")
		default:
			mem, err := NewReMeFileMemory(cfg, counter)
			if err != nil {
				return nil, fmt.Errorf("memory facade: reme file: %w", err)
			}
			f.reme = mem
		}
		f.remeHook = NewReMeHook(f.reme)
	}

	if a := opts.Agentic; a != nil {
		store, err := middleware.NewLocalMemoryStore(a.Dir)
		if err != nil {
			return nil, fmt.Errorf("memory facade: agentic store: %w", err)
		}
		f.mws = append(f.mws, middleware.NewAgenticMemoryMiddleware(store, a.Dir))
	}

	if l := opts.LongTerm; l != nil {
		if vectorReMe == nil {
			return nil, errors.New("memory facade: long-term tier requires a vector-backed ReMe tier (ReMeOptions with Store+Embed)")
		}
		topK := l.TopK
		if topK <= 0 {
			topK = 5
		}
		backend := middleware.NewFuncLongTermMemory(
			func(ctx context.Context, query string, o middleware.SearchOptions) ([]middleware.Memory, error) {
				nodes, err := vectorReMe.RetrieveMemory(ctx, query, RetrieveOptions{TopK: topK})
				if err != nil {
					return nil, err
				}
				out := make([]middleware.Memory, 0, len(nodes))
				for _, n := range nodes {
					if n == nil {
						continue
					}
					out = append(out, middleware.Memory{Text: n.Content, Score: n.Score, Metadata: map[string]any{"id": n.MemoryID}})
				}
				return out, nil
			},
			func(ctx context.Context, texts []string, o middleware.AddOptions) error {
				for _, text := range texts {
					node := &MemoryNode{
						Content:     text,
						MemoryType:  MemoryTypeHistory,
						MessageTime: time.Now(),
						Author:      l.UserID,
					}
					if err := vectorReMe.AddMemory(ctx, node); err != nil {
						return err
					}
				}
				return nil
			},
		)
		f.mws = append(f.mws, middleware.NewLongTermMemoryMiddleware(backend, l.UserID))
	}

	return f, nil
}

// Window returns the in-conversation memory (nil when the tier is off).
func (f *Facade) Window() Memory { return f.window }

// ReMe returns the retrieval memory (nil when the tier is off).
func (f *Facade) ReMe() ReMeMemory { return f.reme }

// Hooks returns the hooks to attach to the agent (the ReMe hook; empty when
// the tier is off).
func (f *Facade) Hooks() []hook.Hook {
	if f.remeHook == nil {
		return nil
	}
	return []hook.Hook{f.remeHook}
}

// Middlewares returns the middleware chain to attach to the agent (Agentic +
// LongTerm; empty when both tiers are off).
func (f *Facade) Middlewares() []middleware.Middleware {
	if len(f.mws) == 0 {
		return nil
	}
	return append([]middleware.Middleware(nil), f.mws...)
}
