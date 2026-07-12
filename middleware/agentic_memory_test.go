package middleware

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

func writeMemFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestStore(t *testing.T) *LocalMemoryStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewLocalMemoryStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLocalMemoryStore_EnsureLayout(t *testing.T) {
	s := newTestStore(t)
	if _, err := os.Stat(filepath.Join(s.Dir, "MEMORY.md")); err != nil {
		t.Fatalf("MEMORY.md not created: %v", err)
	}
	// idempotent
	if err := s.EnsureLayout(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLocalMemoryStore_ListFilesFrontmatter(t *testing.T) {
	s := newTestStore(t)
	writeMemFile(t, s.Dir, "user_role.md", "---\nname: role\ndescription: user is a data scientist\ntype: user\n---\nKnows Go and Python.")
	writeMemFile(t, s.Dir, "feedback_testing.md", "---\nname: testing\ndescription: prefer real DB not mocks\ntype: feedback\n---\nAlways test against real DB.")
	writeMemFile(t, s.Dir, "notes.md", "no frontmatter here")

	files, err := s.ListFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 topic files, got %d", len(files))
	}
	byName := map[string]MemoryFileHeader{}
	for _, f := range files {
		byName[f.Filename] = f
	}
	if byName["user_role.md"].Description != "user is a data scientist" {
		t.Fatalf("description not parsed: %+v", byName["user_role.md"])
	}
	if byName["user_role.md"].Type != "user" {
		t.Fatalf("type not parsed: %+v", byName["user_role.md"])
	}
	if byName["notes.md"].Description != "" {
		t.Fatal("no-frontmatter file should have empty description")
	}
}

func TestParseFrontmatter(t *testing.T) {
	fm := parseFrontmatter("---\nname: x\ndescription: a thing\ntype: project\n---\nbody")
	if fm["name"] != "x" || fm["description"] != "a thing" || fm["type"] != "project" {
		t.Fatalf("unexpected frontmatter: %+v", fm)
	}
	if fm := parseFrontmatter("no frontmatter"); len(fm) != 0 {
		t.Fatalf("expected empty frontmatter, got %+v", fm)
	}
}

func TestKeywordSelector(t *testing.T) {
	files := []MemoryFileHeader{
		{Filename: "a.md", Description: "database testing", Content: "always use real database"},
		{Filename: "b.md", Description: "user role", Content: "data scientist knows go"},
		{Filename: "c.md", Description: "unrelated", Content: "weather report"},
	}
	sel := KeywordSelector(5)
	got, err := sel(context.Background(), "database testing", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0].Filename != "a.md" {
		t.Fatalf("expected a.md first, got %+v", got)
	}
	// 'weather' query matches nothing
	got, _ = sel(context.Background(), "weather", files)
	if len(got) != 1 || got[0].Filename != "c.md" {
		t.Fatalf("expected only c.md, got %+v", got)
	}
	// empty query
	got, _ = sel(context.Background(), "", files)
	if len(got) != 0 {
		t.Fatal("empty query should select nothing")
	}
}

func TestTruncateApproxTokens(t *testing.T) {
	short := "hello"
	if got := truncateApproxTokens(short, 100); got != short {
		t.Fatal("short content should be unchanged")
	}
	long := strings.Repeat("a", 1000)     // 250 tokens
	got := truncateApproxTokens(long, 50) // ~50 tokens = 200 bytes
	if !strings.HasSuffix(got, "<<<TRUNCATED>>>") {
		t.Fatal("expected truncation marker")
	}
	if len(got) > 220 {
		t.Fatalf("truncated too long: %d", len(got))
	}
}

type stubA struct{ name string }

func (s *stubA) AgentName() string { return s.name }

func TestAgenticMemory_OnSystemPrompt(t *testing.T) {
	s := newTestStore(t)
	os.WriteFile(filepath.Join(s.Dir, "MEMORY.md"), []byte("- [Role](user_role.md) — data scientist\n"), 0o644)
	mw := NewAgenticMemoryMiddleware(s, "/mem")
	out, err := mw.OnSystemPrompt(context.Background(), &stubA{"A"}, "base prompt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Auto Memory") {
		t.Fatal("instructions not injected")
	}
	if !strings.Contains(out, "data scientist") {
		t.Fatal("MEMORY.md snapshot not injected")
	}
	if !strings.Contains(out, "/mem") {
		t.Fatal("memory_dir not substituted")
	}
}

func TestAgenticMemory_OnSystemPromptEmpty(t *testing.T) {
	s := newTestStore(t)
	mw := NewAgenticMemoryMiddleware(s, "/mem")
	out, _ := mw.OnSystemPrompt(context.Background(), &stubA{"A"}, "base")
	if !strings.Contains(out, "currently empty") {
		t.Fatalf("empty MEMORY.md should show placeholder: %q", out)
	}
}

func TestAgenticMemory_RetrievalInjectsHint(t *testing.T) {
	s := newTestStore(t)
	writeMemFile(t, s.Dir, "feedback_db.md",
		"---\nname: db\ndescription: prefer real database\ntype: feedback\n---\nAlways test against a real database, not mocks.")
	mw := NewAgenticMemoryMiddleware(s, "/mem").WithSelector(KeywordSelector(5))

	ctx := context.Background()
	var hint *message.HintBlock
	mw.OnReply(ctx, &stubA{"A"}, &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("how should I test the database?").Build(),
	}}, func(ctx context.Context) (*message.Msg, error) {
		// Poll OnReasoning until the async retrieval lands and injects a hint.
		for i := 0; i < 50; i++ {
			rin := &ReasoningInput{Messages: []*message.Msg{}}
			mw.OnReasoning(ctx, &stubA{"A"}, rin, func(ctx context.Context) (*message.Msg, error) {
				return nil, nil
			})
			for _, m2 := range rin.Messages {
				for _, b := range m2.Content {
					if h, ok := b.(*message.HintBlock); ok {
						hint = h
					}
				}
			}
			if hint != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		return nil, nil
	})
	if hint == nil {
		t.Fatal("expected HintBlock injected after retrieval")
	}
	if !strings.Contains(hint.Text, "real database") {
		t.Fatalf("hint missing memory content: %q", hint.Text)
	}
	if hint.Kind != "agentic_memory" {
		t.Fatalf("wrong kind: %q", hint.Kind)
	}
}

func TestAgenticMemory_RetrievalNoMatchNoInjection(t *testing.T) {
	s := newTestStore(t)
	writeMemFile(t, s.Dir, "weather.md",
		"---\nname: w\ndescription: weather\ntype: reference\n---\nIt rains a lot.")
	mw := NewAgenticMemoryMiddleware(s, "/mem")

	injected := false
	mw.OnReply(context.Background(), &stubA{"A"}, &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("database testing").Build(),
	}}, func(ctx context.Context) (*message.Msg, error) {
		time.Sleep(50 * time.Millisecond) // let retrieval finish
		rin := &ReasoningInput{Messages: []*message.Msg{}}
		mw.OnReasoning(ctx, &stubA{"A"}, rin, func(ctx context.Context) (*message.Msg, error) {
			return nil, nil
		})
		for _, m2 := range rin.Messages {
			if len(m2.Content) > 0 {
				injected = true
			}
		}
		return nil, nil
	})
	if injected {
		t.Fatal("should not inject when no memory matches the query")
	}
}

func TestAgenticMemory_InjectedOnlyOnce(t *testing.T) {
	s := newTestStore(t)
	writeMemFile(t, s.Dir, "db.md", "---\nname: db\ndescription: database\ntype: feedback\n---\nUse real DB.")
	mw := NewAgenticMemoryMiddleware(s, "/mem")

	var totalHints int
	mw.OnReply(context.Background(), &stubA{"A"}, &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("database").Build(),
	}}, func(ctx context.Context) (*message.Msg, error) {
		time.Sleep(30 * time.Millisecond)
		for i := 0; i < 5; i++ {
			rin := &ReasoningInput{Messages: []*message.Msg{}}
			mw.OnReasoning(ctx, &stubA{"A"}, rin, func(ctx context.Context) (*message.Msg, error) {
				return nil, nil
			})
			for _, m2 := range rin.Messages {
				for _, b := range m2.Content {
					if _, ok := b.(*message.HintBlock); ok {
						totalHints++
					}
				}
			}
		}
		return nil, nil
	})
	if totalHints != 1 {
		t.Fatalf("expected exactly 1 hint injection across iterations, got %d", totalHints)
	}
}

func TestAgenticMemory_NilStorePassthrough(t *testing.T) {
	mw := &AgenticMemoryMiddleware{} // no store
	out, err := mw.OnSystemPrompt(context.Background(), &stubA{"A"}, "base")
	if err != nil || out != "base" {
		t.Fatal("nil store should pass through system prompt")
	}
	proceeded := false
	mw.OnReply(context.Background(), &stubA{"A"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		proceeded = true
		return nil, nil
	})
	if !proceeded {
		t.Fatal("nil store should still proceed to next")
	}
}
