package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// KnowledgeHit is one retrieved knowledge chunk shown to the agent.
type KnowledgeHit struct {
	Text     string
	Score    float64
	Source   string
	Metadata map[string]any
}

// KnowledgeSearcher searches one or more knowledge bases. The caller adapts a
// rag/kb.KnowledgeBase (or several) into this closure — kept decoupled from
// the rag/kb package, mirroring the LongTermMemory contract.
type KnowledgeSearcher func(ctx context.Context, query string, topK int) ([]KnowledgeHit, error)

// RAGMode controls how the agent accesses knowledge, mirroring the two faces
// of Python's RAGMiddleware: passive injection vs. agent-driven tool calls.
type RAGMode string

const (
	// RAGModeStatic searches the KB before each reply and injects results as a
	// hint. The agent never sees a search tool.
	RAGModeStatic RAGMode = "static"
	// RAGModeAgent exposes a search_knowledge tool for the agent to call on
	// demand. No automatic injection.
	RAGModeAgent RAGMode = "agent"
	// RAGModeBoth enables both automatic injection and the on-demand tool.
	RAGModeBoth RAGMode = "both"
)

const (
	defaultRAGSectionHeader = "## Knowledge base results"
	defaultRAGSectionIntro  = "The following documents from the knowledge base may be relevant. " +
		"Cite the source when you rely on them."
	defaultRAGToolInstructions = "## Knowledge base\n\n" +
		"You have a `search_knowledge` tool. Use it to look up information in the knowledge " +
		"base before answering factual questions."
)

// ragState carries the OnReply search results to the OnReasoning injector.
type ragState struct {
	hits     []KnowledgeHit
	injected bool
}

type ragCtxKey struct{}

// RAGMiddleware adds knowledge-base retrieval to an agent, mirroring Python
// agentscope's RAGMiddleware. In static/both modes it searches on each reply
// and injects the hits as a HintBlock before the first reasoning step. In
// agent/both modes it exposes a search_knowledge tool.
type RAGMiddleware struct {
	Base
	// Searcher retrieves knowledge hits for a query.
	Searcher KnowledgeSearcher
	// Mode selects static / agent / both.
	Mode RAGMode
	// KBName labels the knowledge base in hints (display only).
	KBName string
	// TopK limits retrieved hits per search.
	TopK int
	// MinScore filters out low-relevance hits.
	MinScore float64

	SectionHeader    string
	SectionIntro     string
	ToolInstructions string
}

// NewRAGMiddleware creates a middleware in "both" mode.
func NewRAGMiddleware(searcher KnowledgeSearcher, kbName string) *RAGMiddleware {
	return &RAGMiddleware{
		Searcher:         searcher,
		Mode:             RAGModeBoth,
		KBName:           kbName,
		TopK:             5,
		SectionHeader:    defaultRAGSectionHeader,
		SectionIntro:     defaultRAGSectionIntro,
		ToolInstructions: defaultRAGToolInstructions,
	}
}

// WithMode sets the interaction mode (builder-style).
func (m *RAGMiddleware) WithMode(mode RAGMode) *RAGMiddleware { m.Mode = mode; return m }

// WithTopK sets the max retrieved hits.
func (m *RAGMiddleware) WithTopK(k int) *RAGMiddleware { m.TopK = k; return m }

// WithMinScore sets the minimum relevance score.
func (m *RAGMiddleware) WithMinScore(s float64) *RAGMiddleware { m.MinScore = s; return m }

// Tools returns the search_knowledge tool for agent/both modes.
func (m *RAGMiddleware) Tools() []tool.Tool {
	if m.Mode == RAGModeStatic || m.Searcher == nil {
		return nil
	}
	return []tool.Tool{&ragSearchTool{middleware: m}}
}

// OnReply performs the static search before the reply (static/both modes).
func (m *RAGMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	if m.Mode == RAGModeAgent || m.Searcher == nil {
		return next(ctx)
	}
	query := extractUserQuery(input.Messages)
	var hits []KnowledgeHit
	if query != "" {
		if hs, err := m.Searcher(ctx, query, m.effTopK()); err == nil {
			for _, h := range hs {
				if m.MinScore > 0 && h.Score < m.MinScore {
					continue
				}
				hits = append(hits, h)
			}
		}
	}
	ctx2 := context.WithValue(ctx, ragCtxKey{}, &ragState{hits: hits})
	return next(ctx2)
}

// OnReasoning injects retrieved hits as a HintBlock before the first reasoning
// step (static/both modes only).
func (m *RAGMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	if m.Mode == RAGModeAgent {
		return next(ctx)
	}
	state, _ := ctx.Value(ragCtxKey{}).(*ragState)
	if state == nil || state.injected || len(state.hits) == 0 {
		return next(ctx)
	}
	state.injected = true
	input.Messages = append(input.Messages, buildRAGHintMessage(agent.AgentName(), state.hits, m.SectionHeader, m.SectionIntro))
	return next(ctx)
}

// OnSystemPrompt advertises the search tool in agent/both modes.
func (m *RAGMiddleware) OnSystemPrompt(ctx context.Context, agent Agent, currentPrompt string) (string, error) {
	if m.Mode == RAGModeStatic {
		return currentPrompt, nil
	}
	return currentPrompt + "\n\n" + m.ToolInstructions, nil
}

func (m *RAGMiddleware) effTopK() int {
	if m.TopK <= 0 {
		return 5
	}
	return m.TopK
}

// buildRAGHintMessage formats retrieved hits as an assistant-role hint message
// (user messages cannot carry HintBlock content).
func buildRAGHintMessage(agentName string, hits []KnowledgeHit, header, intro string) *message.Msg {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", header, intro)
	for _, h := range hits {
		b.WriteString("- ")
		if h.Source != "" {
			b.WriteString("[")
			b.WriteString(h.Source)
			b.WriteString("] ")
		}
		b.WriteString(h.Text)
		b.WriteString("\n")
	}
	return message.NewMsg().
		Role(message.RoleAssistant).
		Name(agentName).
		Content(message.NewHintBlock(b.String(), "knowledge_base")).
		Build()
}

// ragSearchTool is the search_knowledge tool exposed in agent/both modes.
type ragSearchTool struct {
	middleware *RAGMiddleware
}

func (t *ragSearchTool) Name() string { return "search_knowledge" }

func (t *ragSearchTool) Description() string {
	return "Search the knowledge base for documents relevant to a query."
}

func (t *ragSearchTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "The search query"},
			},
			"required": []string{"query"},
		},
	}
}

func (t *ragSearchTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("rag: query is required")
	}
	hits, err := t.middleware.Searcher(ctx, query, t.middleware.effTopK())
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return tool.NewTextResponse("No relevant documents found."), nil
	}
	var b strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&b, "[%d] ", i+1)
		if h.Source != "" {
			b.WriteString("(")
			b.WriteString(h.Source)
			b.WriteString(") ")
		}
		b.WriteString(h.Text)
		b.WriteString("\n\n")
	}
	return tool.NewTextResponse(strings.TrimSpace(b.String())), nil
}

// compile-time assertions that RAGMiddleware implements the middleware hooks.
var (
	_ ReplyInterceptor        = (*RAGMiddleware)(nil)
	_ ReasoningInterceptor    = (*RAGMiddleware)(nil)
	_ SystemPromptTransformer = (*RAGMiddleware)(nil)
)
