package memory

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if s := CosineSimilarity(a, b); math.Abs(s-1.0) > 1e-9 {
		t.Fatalf("same vector: %v", s)
	}
	if s := CosineSimilarity(a, []float32{0, 1, 0}); math.Abs(s) > 1e-9 {
		t.Fatalf("orthogonal: %v", s)
	}
	if s := CosineSimilarity(a, []float32{1, 1, 0}); s <= 0 || s >= 1 {
		t.Fatalf("expected (0,1): %v", s)
	}
	if CosineSimilarity(nil, a) != 0 || CosineSimilarity(a, []float32{1}) != 0 {
		t.Fatal("mismatch/empty should be 0")
	}
}
