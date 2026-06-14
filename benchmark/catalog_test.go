package benchmark

import (
	"testing"
)

// TestCatalogIntegrity verifies that the benchmark catalog is non-empty and well-formed.
func TestCatalogIntegrity(t *testing.T) {
	if len(Catalog) == 0 {
		t.Fatal("benchmark catalog is empty")
	}
	for _, cat := range Catalog {
		if cat.Name == "" {
			t.Error("category has empty name")
		}
		if cat.Path == "" {
			t.Errorf("category %s has empty path", cat.Name)
		}
		if len(cat.Benchmarks) == 0 {
			t.Errorf("category %s has no benchmarks listed", cat.Name)
		}
		for i, b := range cat.Benchmarks {
			if b == "" {
				t.Errorf("category %s benchmark[%d] is empty", cat.Name, i)
			}
		}
	}
}

// TestCatalogCoverage checks that we cover at least 5 domains.
func TestCatalogCoverage(t *testing.T) {
	if len(Catalog) < 5 {
		t.Errorf("expected at least 5 benchmark categories, got %d", len(Catalog))
	}
}

func BenchmarkCatalogIteration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, cat := range Catalog {
			for range cat.Benchmarks {
				_ = cat.Name
			}
		}
	}
}
