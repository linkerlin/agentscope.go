package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// MemoryMode controls how an agent interacts with a LongTermMemory backend,
// mirroring Python agentscope's Mem0Middleware modes (#1775).
type MemoryMode string

const (
	// MemoryModeStaticControl searches memories before each reply, injects them
	// into the reasoning context as a hint, and writes the new exchange back
	// afterwards. The agent never sees memory as a tool.
	MemoryModeStaticControl MemoryMode = "static_control"
	// MemoryModeAgentControl exposes search_memory / add_memory tools for the
	// agent to invoke on demand. No automatic retrieval or write-back.
	MemoryModeAgentControl MemoryMode = "agent_control"
	// MemoryModeBoth enables both static retrieval/write-back and on-demand tools.
	MemoryModeBoth MemoryMode = "both"
)

// Memory is a single long-term memory record exposed to the agent.
type Memory struct {
	// Text is the memory content shown to the model.
	Text string
	// Score is the retrieval similarity/relevance score (optional).
	Score float64
	// Metadata carries arbitrary backend-specific metadata.
	Metadata map[string]any
}

// SearchOptions controls a long-term memory search.
type SearchOptions struct {
	// TopK limits the number of returned memories.
	TopK int
	// MinScore filters out memories below this relevance score.
	MinScore float64
	// UserID namespaces memories per user (mem0-style).
	UserID string
	// AgentID optionally namespaces memories per agent.
	AgentID string
}

// AddOptions controls a long-term memory write.
type AddOptions struct {
	UserID   string
	AgentID  string
	Metadata map[string]any
}

// LongTermMemory is the backend contract for the long-term memory middleware.
// It is intentionally decoupled from the heavy memory/ package: wrap any
// backend (ReMe vector memory, mem0 HTTP, a custom store) via NewFuncLongTermMemory.
type LongTermMemory interface {
	// Search retrieves memories relevant to query.
	Search(ctx context.Context, query string, opts SearchOptions) ([]Memory, error)
	// Add persists the given texts as new memories.
	Add(ctx context.Context, texts []string, opts AddOptions) error
}

// FuncLongTermMemory adapts two closures into a LongTermMemory, letting callers
// bridge an existing backend (e.g. memory.ReMeVectorMemory) without a hard
// dependency. Either function may be nil (a no-op returning nil/empty).
type FuncLongTermMemory struct {
	SearchFn func(ctx context.Context, query string, opts SearchOptions) ([]Memory, error)
	AddFn    func(ctx context.Context, texts []string, opts AddOptions) error
}

// NewFuncLongTermMemory builds a LongTermMemory from two closures.
func NewFuncLongTermMemory(search func(ctx context.Context, query string, opts SearchOptions) ([]Memory, error), add func(ctx context.Context, texts []string, opts AddOptions) error) *FuncLongTermMemory {
	return &FuncLongTermMemory{SearchFn: search, AddFn: add}
}

func (f *FuncLongTermMemory) Search(ctx context.Context, query string, opts SearchOptions) ([]Memory, error) {
	if f == nil || f.SearchFn == nil {
		return nil, nil
	}
	return f.SearchFn(ctx, query, opts)
}

func (f *FuncLongTermMemory) Add(ctx context.Context, texts []string, opts AddOptions) error {
	if f == nil || f.AddFn == nil {
		return nil
	}
	return f.AddFn(ctx, texts, opts)
}

// InMemoryLongTermMemory is a simple, thread-safe in-process long-term memory
// store. Search is a case-insensitive substring match returning the TopK most
// recent matches; suitable for demos and tests.
type InMemoryLongTermMemory struct {
	mu     sync.Mutex
	byUser map[string][]Memory
}

// NewInMemoryLongTermMemory creates an empty in-memory long-term memory store.
func NewInMemoryLongTermMemory() *InMemoryLongTermMemory {
	return &InMemoryLongTermMemory{byUser: map[string][]Memory{}}
}

// Search returns up to TopK memories whose Text case-insensitively contains the
// query, scoped to opts.UserID (empty UserID searches the global pool).
func (m *InMemoryLongTermMemory) Search(ctx context.Context, query string, opts SearchOptions) ([]Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pool := m.byUser[opts.UserID]
	needle := strings.ToLower(query)
	var out []Memory
	// Iterate most-recent-first for stable TopK semantics.
	for i := len(pool) - 1; i >= 0 && len(out) < effectiveTopK(opts.TopK); i-- {
		mem := pool[i]
		if needle == "" || strings.Contains(strings.ToLower(mem.Text), needle) {
			if opts.MinScore > 0 && mem.Score < opts.MinScore {
				continue
			}
			out = append(out, mem)
		}
	}
	return out, nil
}

// Add appends texts to the user's memory pool.
func (m *InMemoryLongTermMemory) Add(ctx context.Context, texts []string, opts AddOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range texts {
		m.byUser[opts.UserID] = append(m.byUser[opts.UserID], Memory{Text: t, Metadata: opts.Metadata})
	}
	return nil
}

// Snapshot returns a copy of all memories for a user (test helper).
func (m *InMemoryLongTermMemory) Snapshot(userID string) []Memory {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Memory, len(m.byUser[userID]))
	copy(out, m.byUser[userID])
	return out
}

func effectiveTopK(k int) int {
	if k <= 0 {
		return 5
	}
	return k
}

// Default memory-injection copy used by the static-control path.
const (
	defaultLTMSectionHeader = "## Relevant memories from past conversations"
	defaultLTMSectionIntro  = "The following memories about the user may be relevant. " +
		"Use them only if they are pertinent to the current request."
	defaultLTMToolInstructions = "## Long-term memory\n\n" +
		"You have `search_memory` and `add_memory` tools available. Use them whenever the " +
		"conversation depends on (search) or contributes (add) a durable fact about the user."
)

// ltmState is carried via context so OnReply (search) and OnReasoning (inject)
// share per-reply memory state. The reasoning iterations are sequential within
// a reply, so the injected flag needs no extra synchronization.
type ltmState struct {
	memories []Memory
	injected bool
}

type ltmCtxKey struct{}

// LongTermMemoryMiddleware adds long-term memory to an agent, mirroring Python
// agentscope's Mem0Middleware (#1775). It supports three modes (see MemoryMode).
//
// In static_control / both modes it:
//  1. Searches the backend before the reply using the user's query.
//  2. Injects retrieved memories as a hint message before the first reasoning
//     step (via the on_reasoning hook and the framework's input propagation).
//  3. Writes the completed user/assistant exchange back after the reply.
//
// In agent_control / both modes it advertises search_memory / add_memory tools
// (obtainable via Tools()) and appends a tool-instruction block to the system
// prompt.
type LongTermMemoryMiddleware struct {
	Base
	// Backend is the long-term memory store.
	Backend LongTermMemory
	// Mode selects static_control / agent_control / both.
	Mode MemoryMode
	// UserID namespaces memories (mem0-style). Required for static write-back.
	UserID string
	// AgentID optionally namespaces memories per agent.
	AgentID string
	// TopK is the max memories retrieved per static-control search.
	TopK int
	// MinScore filters low-relevance memories.
	MinScore float64
	// AwaitWrite, when true (default), awaits the post-turn write inline.
	AwaitWrite bool

	SectionHeader    string
	SectionIntro     string
	ToolInstructions string
}

// NewLongTermMemoryMiddleware creates a middleware in "both" mode with the
// given backend and user id.
func NewLongTermMemoryMiddleware(backend LongTermMemory, userID string) *LongTermMemoryMiddleware {
	return &LongTermMemoryMiddleware{
		Backend:          backend,
		Mode:             MemoryModeBoth,
		UserID:           userID,
		TopK:             5,
		AwaitWrite:       true,
		SectionHeader:    defaultLTMSectionHeader,
		SectionIntro:     defaultLTMSectionIntro,
		ToolInstructions: defaultLTMToolInstructions,
	}
}

// WithMode sets the interaction mode (builder-style).
func (m *LongTermMemoryMiddleware) WithMode(mode MemoryMode) *LongTermMemoryMiddleware {
	m.Mode = mode
	return m
}

// WithAgentID sets the agent namespace (builder-style).
func (m *LongTermMemoryMiddleware) WithAgentID(id string) *LongTermMemoryMiddleware {
	m.AgentID = id
	return m
}

// WithTopK sets the max retrieved memories (builder-style).
func (m *LongTermMemoryMiddleware) WithTopK(k int) *LongTermMemoryMiddleware {
	m.TopK = k
	return m
}

// Tools returns the memory tools for agent_control / both modes (empty
// otherwise), ready to register in an agent's toolkit.
func (m *LongTermMemoryMiddleware) Tools() []tool.Tool {
	if m.Mode == MemoryModeStaticControl {
		return nil
	}
	return []tool.Tool{NewMemorySearchTool(m.Backend, m), NewMemoryAddTool(m.Backend, m)}
}

// OnReply performs the static-control search before the reply and the write-back
// after it. In pure agent_control mode it is a pass-through.
func (m *LongTermMemoryMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	if m.Mode == MemoryModeAgentControl || m.Backend == nil {
		return next(ctx)
	}

	query := extractUserQuery(input.Messages)
	var memories []Memory
	if query != "" {
		if ms, err := m.Backend.Search(ctx, query, SearchOptions{
			TopK:     m.TopK,
			MinScore: m.MinScore,
			UserID:   m.UserID,
			AgentID:  m.AgentID,
		}); err == nil {
			memories = ms
		}
	}

	state := &ltmState{memories: memories}
	ctx2 := context.WithValue(ctx, ltmCtxKey{}, state)
	msg, err := next(ctx2)

	// Write-back the completed exchange.
	if m.AwaitWrite && err == nil && msg != nil && query != "" {
		assistantText := msg.GetTextContent()
		if assistantText != "" {
			_ = m.Backend.Add(ctx2, []string{query, assistantText}, AddOptions{
				UserID:  m.UserID,
				AgentID: m.AgentID,
			})
		}
	}
	return msg, err
}

// OnReasoning injects the retrieved memories as a hint message before the first
// reasoning step (static_control / both modes only).
func (m *LongTermMemoryMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	if m.Mode == MemoryModeAgentControl {
		return next(ctx)
	}
	state, _ := ctx.Value(ltmCtxKey{}).(*ltmState)
	if state == nil || state.injected || len(state.memories) == 0 {
		return next(ctx)
	}
	state.injected = true
	input.Messages = append(input.Messages, buildMemoryHintMessage(agent.AgentName(), state.memories, m.SectionHeader, m.SectionIntro))
	return next(ctx)
}

// OnSystemPrompt advertises the memory tools to the LLM in agent_control / both.
func (m *LongTermMemoryMiddleware) OnSystemPrompt(ctx context.Context, agent Agent, currentPrompt string) (string, error) {
	if m.Mode == MemoryModeStaticControl {
		return currentPrompt, nil
	}
	return currentPrompt + "\n\n" + m.ToolInstructions, nil
}

// extractUserQuery returns the text of the last user message in the input.
func extractUserQuery(msgs []*message.Msg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role == message.RoleUser {
			return msgs[i].GetTextContent()
		}
	}
	return ""
}

// buildMemoryHintMessage formats retrieved memories as an assistant-role hint
// message carrying a HintBlock (user messages cannot carry HintBlock content).
func buildMemoryHintMessage(agentName string, memories []Memory, header, intro string) *message.Msg {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", header, intro)
	for _, mem := range memories {
		b.WriteString("- ")
		b.WriteString(mem.Text)
		b.WriteString("\n")
	}
	return message.NewMsg().
		Role(message.RoleAssistant).
		Name(agentName).
		Content(message.NewHintBlock(b.String(), "long_term_memory")).
		Build()
}
