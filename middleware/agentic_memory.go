package middleware

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/linkerlin/agentscope.go/message"
)

// MemoryFileHeader is the metadata + body of one agentic memory file.
type MemoryFileHeader struct {
	Filename    string // relative path under the memory directory (e.g. user_role.md)
	Description string // one-line description from frontmatter
	Type        string // memory type tag from frontmatter (user/feedback/project/reference)
	Content     string // full Markdown body (for keyword matching / injection)
}

// MemoryStore manages the filesystem layout of an agentic memory system:
// a MEMORY.md index file plus topic *.md files. Backends: LocalMemoryStore.
type MemoryStore interface {
	// EnsureLayout creates the memory directory and an empty MEMORY.md if absent.
	EnsureLayout(ctx context.Context) error
	// ReadMemoryMD returns the MEMORY.md index content ("" if absent).
	ReadMemoryMD(ctx context.Context) (string, error)
	// ListFiles scans topic *.md files (excluding MEMORY.md), parsed frontmatter.
	ListFiles(ctx context.Context) ([]MemoryFileHeader, error)
}

// LocalMemoryStore is a filesystem-backed MemoryStore rooted at Dir.
type LocalMemoryStore struct {
	Dir string
}

// NewLocalMemoryStore creates a store at dir, ensuring the layout exists.
func NewLocalMemoryStore(dir string) (*LocalMemoryStore, error) {
	s := &LocalMemoryStore{Dir: dir}
	if err := s.EnsureLayout(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

const memoryMDFilename = "MEMORY.md"

func (s *LocalMemoryStore) EnsureLayout(ctx context.Context) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	idx := filepath.Join(s.Dir, memoryMDFilename)
	if _, err := os.Stat(idx); os.IsNotExist(err) {
		return os.WriteFile(idx, nil, 0o644)
	}
	return nil
}

func (s *LocalMemoryStore) ReadMemoryMD(ctx context.Context) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, memoryMDFilename))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

func (s *LocalMemoryStore) ListFiles(ctx context.Context) ([]MemoryFileHeader, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	var headers []MemoryFileHeader
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == memoryMDFilename {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		body := string(raw)
		fm := parseFrontmatter(body)
		headers = append(headers, MemoryFileHeader{
			Filename:    e.Name(),
			Description: fm["description"],
			Type:        fm["type"],
			Content:     body,
		})
	}
	return headers, nil
}

// FileSelector decides which memory files are relevant to a query. The default
// (KeywordSelector) matches query terms against filename/description/body.
// Provide an LLM-backed selector to mirror Python's LLM retrieval.
type FileSelector func(ctx context.Context, query string, files []MemoryFileHeader) ([]MemoryFileHeader, error)

// KeywordSelector scores files by counting query-term hits in their filename,
// description, and body. Returns the top maxFiles by score.
func KeywordSelector(maxFiles int) FileSelector {
	if maxFiles <= 0 {
		maxFiles = 5
	}
	return func(ctx context.Context, query string, files []MemoryFileHeader) ([]MemoryFileHeader, error) {
		terms := tokenize(query)
		if len(terms) == 0 {
			return nil, nil
		}
		type scored struct {
			h MemoryFileHeader
			n int
		}
		var ss []scored
		for _, f := range files {
			hay := strings.ToLower(f.Filename + " " + f.Description + " " + f.Content)
			n := 0
			for _, t := range terms {
				n += strings.Count(hay, t)
			}
			if n > 0 {
				ss = append(ss, scored{f, n})
			}
		}
		sort.Slice(ss, func(i, j int) bool { return ss[i].n > ss[j].n })
		if len(ss) > maxFiles {
			ss = ss[:maxFiles]
		}
		out := make([]MemoryFileHeader, len(ss))
		for i, s := range ss {
			out[i] = s.h
		}
		return out, nil
	}
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r < 0x80
	}) {
		if len(f) > 1 {
			out = append(out, f)
		}
	}
	return out
}

// frontmatter: leading ---\nkey: value\n---\nbody
var (
	frontmatterRe = regexp.MustCompile(`(?s)^\s*---\s*\n(.*?)\n---\s*\n`)
	fieldRe       = regexp.MustCompile(`(?m)^(\w+)\s*:\s*(.+)$`)
)

func parseFrontmatter(content string) map[string]string {
	out := map[string]string{}
	m := frontmatterRe.FindStringSubmatch(content)
	if m == nil {
		return out
	}
	for _, fm := range fieldRe.FindAllStringSubmatch(m[1], -1) {
		out[fm[1]] = strings.TrimSpace(fm[2])
	}
	return out
}

// DefaultAgenticMemoryInstructions teaches the agent the agentic-memory
// contract: what to save (user/feedback/project/reference), what NOT to save,
// and how (frontmatter files + MEMORY.md index). Trimmed from Python's spec.
const DefaultAgenticMemoryInstructions = `# Auto Memory

You have a persistent, file-based memory system at {memory_dir}. It already exists — write to it directly with your file tools.

Build up this memory over time so future conversations have a complete picture of who the user is, how they like to collaborate, and the context behind their work.

If the user asks you to remember something, save it immediately. If they ask you to forget something, find and remove the relevant entry.

## Types of memory
- user: the user's role, goals, knowledge, preferences. Save when you learn details that should shape future responses.
- feedback: guidance on how to approach work — both corrections ("don't do X") and confirmations ("yes, keep doing that"). Include WHY so you can judge edge cases later.
- project: ongoing work, goals, decisions, incidents not derivable from code/git. Convert relative dates to absolute dates.
- reference: pointers to external systems (e.g. "bugs tracked in Linear project X").

## What NOT to save
- Code patterns, architecture, file paths, project structure — derivable from the repo.
- Git history or who-changed-what — git log/blame is authoritative.
- Debugging recipes — the fix is in the code.
- Ephemeral task details only useful in the current conversation.

## How to save
Step 1 — write the memory to its own file (e.g. user_role.md) with frontmatter:
---
name: {memory name}
description: {one-line, specific — used to decide future relevance}
type: {user|feedback|project|reference}
---
{memory content; for feedback/project include Why: and How to apply:}

Step 2 — add a one-line pointer to MEMORY.md: - [Title](file.md) — hook.
MEMORY.md is an index, not memory content. Keep it concise.

## When to access
- When memories seem relevant or the user references prior work.
- You MUST check memory when the user asks to recall or remember.
- Memory can be stale. Verify against current state before acting on it. Update stale memories.`

// agenticState carries the in-flight retrieval result from OnReply to OnReasoning.
type agenticState struct {
	result   chan string // HintBlock text (closed/sent when retrieval done)
	injected bool
}

type agenticCtxKey struct{}

// AgenticMemoryMiddleware gives an agent a self-managed, filesystem-backed
// long-term memory. It mirrors Python agentscope's AgenticMemoryMiddleware:
// the agent itself decides what to remember by writing Markdown files (using
// its own file tools + the injected instructions), while the middleware keeps
// a bounded MEMORY.md index in the system prompt and asynchronously surfaces
// relevant memory files as a HintBlock during the reasoning loop.
//
// Unlike LongTermMemoryMiddleware (which exposes search/add tools or does
// passive auto-retrieval), here the agent owns the memory via its native file
// tools — the middleware only sets up the store, the instructions, and the
// retrieval hint.
type AgenticMemoryMiddleware struct {
	Base
	// Store is the filesystem memory backend.
	Store MemoryStore
	// Selector picks relevant files for the current query. nil = KeywordSelector(5).
	Selector FileSelector
	// Instructions appended to the system prompt (with {memory_dir} substituted).
	Instructions string
	// MemoryDir is the path shown to the agent in instructions.
	MemoryDir string
	// MemoryMaxTokens bounds the MEMORY.md snapshot injected into the prompt.
	MemoryMaxTokens int
	// MaxRetrieveFiles caps the files surfaced per retrieval.
	MaxRetrieveFiles int
	// Async enables concurrent retrieval during the reply (default true).
	Async bool
}

// NewAgenticMemoryMiddleware creates a middleware over store with defaults.
func NewAgenticMemoryMiddleware(store MemoryStore, memoryDir string) *AgenticMemoryMiddleware {
	return &AgenticMemoryMiddleware{
		Store:            store,
		MemoryDir:        memoryDir,
		Instructions:     DefaultAgenticMemoryInstructions,
		MemoryMaxTokens:  4000,
		MaxRetrieveFiles: 5,
		Async:            true,
	}
}

// WithSelector sets a custom (e.g. LLM-backed) file selector.
func (m *AgenticMemoryMiddleware) WithSelector(s FileSelector) *AgenticMemoryMiddleware {
	m.Selector = s
	return m
}

func (m *AgenticMemoryMiddleware) selector() FileSelector {
	if m.Selector != nil {
		return m.Selector
	}
	return KeywordSelector(m.MaxRetrieveFiles)
}

// OnSystemPrompt appends memory instructions + a bounded MEMORY.md snapshot.
func (m *AgenticMemoryMiddleware) OnSystemPrompt(ctx context.Context, agent Agent, currentPrompt string) (string, error) {
	if m.Store == nil {
		return currentPrompt, nil
	}
	if err := m.Store.EnsureLayout(ctx); err != nil {
		return currentPrompt, err
	}
	md, _ := m.Store.ReadMemoryMD(ctx)
	snapshot := truncateApproxTokens(md, m.MemoryMaxTokens)
	if snapshot == "" {
		snapshot = "(Your MEMORY.md is currently empty. New memories you save will appear here.)"
	}
	instr := strings.ReplaceAll(m.Instructions, "{memory_dir}", m.MemoryDir)
	return currentPrompt + "\n\n" + instr + "\n\n## MEMORY.md\n" + snapshot, nil
}

// OnReply caches the user query and kicks off async retrieval.
func (m *AgenticMemoryMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	if m.Store == nil {
		return next(ctx)
	}
	query := extractUserQuery(input.Messages)
	state := &agenticState{result: make(chan string, 1)}
	if m.Async && query != "" {
		go m.runRetrieval(context.Background(), query, state)
	}
	ctx2 := context.WithValue(ctx, agenticCtxKey{}, state)
	return next(ctx2)
}

// OnReasoning injects the retrieval result as a HintBlock once it lands.
func (m *AgenticMemoryMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	state, _ := ctx.Value(agenticCtxKey{}).(*agenticState)
	if state != nil && !state.injected {
		select {
		case text := <-state.result:
			state.injected = true
			if text != "" {
				input.Messages = append(input.Messages, message.NewMsg().
					Role(message.RoleAssistant).
					Name(agent.AgentName()).
					Content(message.NewHintBlock(text, "agentic_memory")).
					Build())
			}
		default:
			// retrieval still in flight; try next iteration
		}
	}
	return next(ctx)
}

func (m *AgenticMemoryMiddleware) runRetrieval(ctx context.Context, query string, state *agenticState) {
	defer close(state.result)
	files, err := m.Store.ListFiles(ctx)
	if err != nil || len(files) == 0 {
		return
	}
	selected, err := m.selector()(ctx, query, files)
	if err != nil || len(selected) == 0 {
		return
	}
	var b strings.Builder
	for _, f := range selected {
		b.WriteString(f.Content)
		b.WriteString("\n\n---\n\n")
	}
	state.result <- strings.TrimRight(b.String(), "\n")
}

// truncateApproxTokens caps content to ~maxTokens tokens (utf8 bytes/4).
func truncateApproxTokens(content string, maxTokens int) string {
	if maxTokens <= 0 || content == "" {
		return content
	}
	const bytesPerToken = 4
	if len(content)/bytesPerToken <= maxTokens {
		return content
	}
	// byte budget, snapped to a rune boundary
	budget := maxTokens * bytesPerToken
	if budget > len(content) {
		budget = len(content)
	}
	for budget > 0 && !utf8.RuneStart(content[budget]) {
		budget--
	}
	return content[:budget] + "\n<<<TRUNCATED>>>"
}

// compile-time assertions
var (
	_ SystemPromptTransformer = (*AgenticMemoryMiddleware)(nil)
	_ ReplyInterceptor        = (*AgenticMemoryMiddleware)(nil)
	_ ReasoningInterceptor    = (*AgenticMemoryMiddleware)(nil)
)
