// memory/facade_test.go — 20.8: the four tiers assemble from ONE entry;
// tiers are optional; the long-term bridge runs over the facade's vector
// ReMe tier; partial vector config fails loudly.
package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/middleware"
)

// fakeFacadeStore is an in-package vector store double (no embedding calls).
type fakeFacadeStore struct {
	nodes []*MemoryNode
}

func (s *fakeFacadeStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	s.nodes = append(s.nodes, nodes...)
	return nil
}
func (s *fakeFacadeStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return s.nodes, nil
}
func (s *fakeFacadeStore) Get(ctx context.Context, id string) (*MemoryNode, error) { return nil, nil }
func (s *fakeFacadeStore) Update(ctx context.Context, n *MemoryNode) error         { return nil }
func (s *fakeFacadeStore) Delete(ctx context.Context, id string) error             { return nil }
func (s *fakeFacadeStore) DeleteAll(ctx context.Context) error {
	s.nodes = nil
	return nil
}

// fakeFacadeEmbed is a deterministic embedding model.
type fakeFacadeEmbed struct{}

func (fakeFacadeEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (fakeFacadeEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2}
	}
	return out, nil
}

// facadeLongTermBackend digs the long-term middleware's backend out of the
// facade's middleware chain (it is the FuncLongTermMemory bridge over the
// facade's own vector ReMe tier).
func facadeLongTermBackend(t *testing.T, f *Facade) middleware.LongTermMemory {
	t.Helper()
	for _, mw := range f.Middlewares() {
		if ltm, ok := mw.(*middleware.LongTermMemoryMiddleware); ok {
			return ltm.Backend
		}
	}
	t.Fatal("long-term middleware not in chain")
	return nil
}

// TestFacade_AllTiers: window+ReMe(vector)+Agentic+LongTerm assemble from
// one entry; every wiring point is non-nil and typed.
func TestFacade_AllTiers(t *testing.T) {
	dir := t.TempDir()
	f, err := NewFacade(FacadeOptions{
		Window:   &WindowOptions{MaxMessages: 10},
		ReMe:     &ReMeOptions{WorkingDir: dir, Store: &fakeFacadeStore{}, Embed: fakeFacadeEmbed{}},
		Agentic:  &AgenticOptions{Dir: dir},
		LongTerm: &LongTermOptions{UserID: "u1"},
	})
	if err != nil {
		t.Fatalf("facade: %v", err)
	}
	if f.Window() == nil {
		t.Fatal("window tier missing")
	}
	if err := f.Window().Add(nil); err != nil {
		t.Fatalf("window add: %v", err)
	}
	if f.ReMe() == nil {
		t.Fatal("reme tier missing")
	}
	if len(f.Hooks()) != 1 {
		t.Fatalf("reme hook: %v", f.Hooks())
	}
	if len(f.Middlewares()) != 2 {
		t.Fatalf("middlewares (agentic+longterm): %d", len(f.Middlewares()))
	}
	if facadeLongTermBackend(t, f) == nil {
		t.Fatal("long-term backend missing")
	}
}

// TestFacade_AllOff: an empty config assembles an empty facade.
func TestFacade_AllOff(t *testing.T) {
	f, err := NewFacade(FacadeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Window() != nil || f.ReMe() != nil || f.Hooks() != nil || f.Middlewares() != nil {
		t.Fatalf("empty facade must stay empty: %+v", f)
	}
}

// TestFacade_LongTermBridgesVectorReMe: the long-term middleware's Add/Search
// run through the facade's vector store (add then retrieve round-trip).
func TestFacade_LongTermBridgesVectorReMe(t *testing.T) {
	dir := t.TempDir()
	store := &fakeFacadeStore{}
	f, err := NewFacade(FacadeOptions{
		ReMe:     &ReMeOptions{WorkingDir: dir, Store: store, Embed: fakeFacadeEmbed{}},
		LongTerm: &LongTermOptions{UserID: "u1", TopK: 3},
	})
	if err != nil {
		t.Fatalf("facade: %v", err)
	}

	backend := facadeLongTermBackend(t, f)
	if err := backend.Add(context.Background(), []string{"用户偏好深色主题"}, middleware.AddOptions{}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(store.nodes) != 1 {
		t.Fatalf("add did not reach the vector store: %d nodes", len(store.nodes))
	}
	if got := store.nodes[0]; got.Content != "用户偏好深色主题" || got.Author != "u1" || got.MemoryType != MemoryTypeHistory {
		t.Fatalf("added node mismatch: %+v", got)
	}

	hits, err := backend.Search(context.Background(), "主题偏好", middleware.SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Text != "用户偏好深色主题" {
		t.Fatalf("search did not round-trip: %+v", hits)
	}
	if hits[0].Metadata["id"] != store.nodes[0].MemoryID {
		t.Fatalf("search metadata missing node id: %+v", hits[0].Metadata)
	}
}

// TestFacade_LongTermNeedsVectorTier: long-term without a vector ReMe tier
// fails loudly instead of silently no-op'ing.
func TestFacade_LongTermNeedsVectorTier(t *testing.T) {
	if _, err := NewFacade(FacadeOptions{
		ReMe:     &ReMeOptions{WorkingDir: t.TempDir()}, // file-backed only
		LongTerm: &LongTermOptions{UserID: "u1"},
	}); err == nil {
		t.Fatal("long-term over file-only ReMe must fail")
	}
}

// TestFacade_PartialVectorConfigFails: Store without Embed (or vice versa)
// is rejected explicitly.
func TestFacade_PartialVectorConfigFails(t *testing.T) {
	if _, err := NewFacade(FacadeOptions{
		ReMe: &ReMeOptions{WorkingDir: t.TempDir(), Store: &fakeFacadeStore{}},
	}); err == nil {
		t.Fatal("Store without Embed must fail")
	}
	if _, err := NewFacade(FacadeOptions{
		ReMe: &ReMeOptions{WorkingDir: t.TempDir(), Embed: fakeFacadeEmbed{}},
	}); err == nil {
		t.Fatal("Embed without Store must fail")
	}
}
