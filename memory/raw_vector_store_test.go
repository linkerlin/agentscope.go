package memory

import (
	"context"
	"testing"
)

func TestRawVectorIDStoreSearch(t *testing.T) {
	s := NewRawVectorIDStore()
	_ = s.Upsert(context.Background(), "a", []float32{1, 0, 0}, map[string]any{"text": "a"})
	_ = s.Upsert(context.Background(), "b", []float32{0.9, 0.1, 0}, map[string]any{"text": "b"})
	ids, err := s.Search(context.Background(), []float32{1, 0, 0}, 1)
	if err != nil || len(ids) != 1 || ids[0] != "a" {
		t.Fatal(ids, err)
	}
}
