package vector

import (
	"context"
	"testing"
	"time"
)

// mockEmbedder for SQLiteVec tests.
type sqliteMockEmbedder struct {
	dim int
}

func (m *sqliteMockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return makeVec(text, m.dim), nil
}

func (m *sqliteMockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = makeVec(t, m.dim)
	}
	return out, nil
}

func makeVec(text string, dim int) []float32 {
	v := make([]float32, dim)
	for i := 0; i < dim; i++ {
		v[i] = float32(len(text) + i)
	}
	return v
}

func newTestStore(t *testing.T, dim int) *SQLiteVecStore {
	t.Helper()
	s, err := NewSQLiteVecStore(":memory:", dim, &sqliteMockEmbedder{dim: dim})
	if err != nil {
		t.Fatalf("NewSQLiteVecStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteVec_InsertAndGet(t *testing.T) {
	s := newTestStore(t, 8)

	node := &MemoryNode{
		MemoryID:   "test-001",
		MemoryType: MemoryTypePersonal,
		Content:    "I love Go programming",
		WhenToUse:  "when discussing languages",
		Author:     "tester",
		Metadata:   map[string]any{"lang": "go"},
	}

	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := s.Get(context.Background(), "test-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "I love Go programming" {
		t.Errorf("unexpected content: %s", got.Content)
	}
	if got.Author != "tester" {
		t.Errorf("unexpected author: %s", got.Author)
	}
	if got.Metadata["lang"] != "go" {
		t.Errorf("unexpected metadata: %v", got.Metadata)
	}
}

func TestSQLiteVec_InsertAutoEmbed(t *testing.T) {
	s := newTestStore(t, 4)

	node := &MemoryNode{
		MemoryID:   "auto-001",
		Content:    "auto embedded content",
		MemoryType: MemoryTypeProcedural,
	}

	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if len(node.Vector) != 4 {
		t.Fatalf("expected auto-embedded 4-dim vector, got %d", len(node.Vector))
	}
}

func TestSQLiteVec_InsertAutoID(t *testing.T) {
	s := newTestStore(t, 4)

	node := &MemoryNode{
		Content:    "no id provided",
		MemoryType: MemoryTypeProfile,
	}

	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if node.MemoryID == "" {
		t.Fatal("expected auto-generated MemoryID")
	}

	got, err := s.Get(context.Background(), node.MemoryID)
	if err != nil {
		t.Fatalf("Get by auto ID: %v", err)
	}
	if got.Content != "no id provided" {
		t.Errorf("unexpected content: %s", got.Content)
	}
}

func TestSQLiteVec_InsertDimensionMismatch(t *testing.T) {
	s := newTestStore(t, 4)

	node := &MemoryNode{
		MemoryID: "bad-dim",
		Content:  "wrong dimension",
		Vector:   []float32{1, 2, 3},
	}

	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}

func TestSQLiteVec_Search(t *testing.T) {
	s := newTestStore(t, 4)

	nodes := []*MemoryNode{
		{MemoryID: "s1", Content: "golang programming", MemoryType: MemoryTypePersonal},
		{MemoryID: "s2", Content: "golang testing", MemoryType: MemoryTypeProcedural},
		{MemoryID: "s3", Content: "python scripting", MemoryType: MemoryTypePersonal},
	}
	err := s.Insert(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Search(context.Background(), "golang programming", RetrieveOptions{TopK: 3})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].MemoryID != "s1" {
		t.Errorf("expected s1 first, got %s", results[0].MemoryID)
	}
	if results[0].Score <= 0.5 {
		t.Errorf("expected high similarity score, got %f", results[0].Score)
	}
}

func TestSQLiteVec_SearchWithMemoryTypeFilter(t *testing.T) {
	s := newTestStore(t, 4)

	nodes := []*MemoryNode{
		{MemoryID: "f1", Content: "same text", MemoryType: MemoryTypePersonal},
		{MemoryID: "f2", Content: "same text", MemoryType: MemoryTypeProcedural},
		{MemoryID: "f3", Content: "same text", MemoryType: MemoryTypeProfile},
	}
	err := s.Insert(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Search(context.Background(), "same text", RetrieveOptions{
		TopK:        10,
		MemoryTypes: []MemoryType{MemoryTypePersonal},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (personal only), got %d", len(results))
	}
	if results[0].MemoryID != "f1" {
		t.Errorf("expected f1, got %s", results[0].MemoryID)
	}
}

func TestSQLiteVec_SearchWithMinScore(t *testing.T) {
	s := newTestStore(t, 4)

	nodes := []*MemoryNode{
		{MemoryID: "m1", Content: "aaa", MemoryType: MemoryTypePersonal},
		{MemoryID: "m2", Content: "zzz", MemoryType: MemoryTypePersonal},
	}
	err := s.Insert(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Search(context.Background(), "aaa", RetrieveOptions{
		TopK:     10,
		MinScore: 0.99,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	for _, r := range results {
		if r.Score < 0.99 {
			t.Errorf("result %s score %f below min 0.99", r.MemoryID, r.Score)
		}
	}
}

func TestSQLiteVec_Update(t *testing.T) {
	s := newTestStore(t, 4)

	node := &MemoryNode{
		MemoryID:   "u1",
		Content:    "original content",
		MemoryType: MemoryTypePersonal,
	}
	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	node.Content = "updated content"
	node.Author = "editor"
	node.Vector = []float32{5, 6, 7, 8}
	err = s.Update(context.Background(), node)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Content != "updated content" {
		t.Errorf("expected updated content, got '%s'", got.Content)
	}
	if got.Author != "editor" {
		t.Errorf("expected author 'editor', got '%s'", got.Author)
	}
}

func TestSQLiteVec_Delete(t *testing.T) {
	s := newTestStore(t, 4)

	node := &MemoryNode{
		MemoryID:   "d1",
		Content:    "to be deleted",
		MemoryType: MemoryTypePersonal,
	}
	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	err = s.Delete(context.Background(), "d1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(context.Background(), "d1")
	if err != ErrMemoryNotFound {
		t.Errorf("expected ErrMemoryNotFound, got %v", err)
	}
}

func TestSQLiteVec_DeleteAll(t *testing.T) {
	s := newTestStore(t, 4)

	nodes := []*MemoryNode{
		{MemoryID: "da1", Content: "first", MemoryType: MemoryTypePersonal},
		{MemoryID: "da2", Content: "second", MemoryType: MemoryTypePersonal},
	}
	err := s.Insert(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	err = s.DeleteAll(context.Background())
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	results, err := s.Search(context.Background(), "first", RetrieveOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search after deleteall: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after DeleteAll, got %d", len(results))
	}
}

func TestSQLiteVec_GetNotFound(t *testing.T) {
	s := newTestStore(t, 4)

	_, err := s.Get(context.Background(), "nonexistent")
	if err != ErrMemoryNotFound {
		t.Errorf("expected ErrMemoryNotFound, got %v", err)
	}
}

func TestSQLiteVec_PreservesTimeFields(t *testing.T) {
	s := newTestStore(t, 4)

	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	msgTime := time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC)

	node := &MemoryNode{
		MemoryID:    "t1",
		Content:     "time test",
		MemoryType:  MemoryTypePersonal,
		TimeCreated: created,
		MessageTime: msgTime,
	}
	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := s.Get(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.TimeCreated.Equal(created) {
		t.Errorf("TimeCreated: expected %v, got %v", created, got.TimeCreated)
	}
	if !got.MessageTime.Equal(msgTime) {
		t.Errorf("MessageTime: expected %v, got %v", msgTime, got.MessageTime)
	}
}

func TestSQLiteVec_ReplacesExisting(t *testing.T) {
	s := newTestStore(t, 4)

	node := &MemoryNode{
		MemoryID:   "r1",
		Content:    "first version",
		MemoryType: MemoryTypePersonal,
	}
	err := s.Insert(context.Background(), []*MemoryNode{node})
	if err != nil {
		t.Fatalf("Insert first: %v", err)
	}

	node2 := &MemoryNode{
		MemoryID:   "r1",
		Content:    "second version",
		MemoryType: MemoryTypePersonal,
	}
	err = s.Insert(context.Background(), []*MemoryNode{node2})
	if err != nil {
		t.Fatalf("Insert replace: %v", err)
	}

	got, err := s.Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "second version" {
		t.Errorf("expected replaced content, got '%s'", got.Content)
	}

	results, err := s.Search(context.Background(), "test", RetrieveOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (replaced, not duplicated), got %d", len(results))
	}
}

func TestSQLiteVec_EmbedRequired(t *testing.T) {
	_, err := NewSQLiteVecStore(":memory:", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Errorf("expected ErrEmbeddingRequired, got %v", err)
	}
}

func TestSQLiteVec_InvalidDim(t *testing.T) {
	_, err := NewSQLiteVecStore(":memory:", 0, &sqliteMockEmbedder{dim: 4})
	if err == nil {
		t.Fatal("expected error for dim=0")
	}
}

func TestSQLiteVec_NewFromDB(t *testing.T) {
	s1 := newTestStore(t, 4)

	err := s1.Insert(context.Background(), []*MemoryNode{
		{MemoryID: "db1", Content: "shared db test", MemoryType: MemoryTypePersonal},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	s2, err := NewSQLiteVecStoreFromDB(s1.db, 4, &sqliteMockEmbedder{dim: 4})
	if err != nil {
		t.Fatalf("NewSQLiteVecStoreFromDB: %v", err)
	}

	got, err := s2.Get(context.Background(), "db1")
	if err != nil {
		t.Fatalf("Get from shared DB: %v", err)
	}
	if got.Content != "shared db test" {
		t.Errorf("unexpected content: %s", got.Content)
	}
}

func TestNormalizeVector(t *testing.T) {
	v := []float32{3, 4}
	normalizeVector(v)
	if abs(float64(v[0])-0.6) > 0.001 || abs(float64(v[1])-0.8) > 0.001 {
		t.Errorf("expected [0.6, 0.8], got %v", v)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
