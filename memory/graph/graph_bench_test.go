package graph

import (
	"fmt"
	"testing"
)

func BenchmarkFindAllPaths(b *testing.B) {
	g := NewGraph()
	for i := 0; i < 20; i++ {
		_ = g.AddNode(&Node{ID: fmt.Sprintf("n%d", i), Title: fmt.Sprintf("Node%d", i)})
	}
	for i := 0; i < 19; i++ {
		_ = g.AddEdge(&Edge{Source: fmt.Sprintf("n%d", i), Target: fmt.Sprintf("n%d", i+1), Relation: RelRelatedTo})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.FindAllPaths("n0", "n19", 25, 10)
	}
}

func BenchmarkMultiHopNeighbors(b *testing.B) {
	g := NewGraph()
	for i := 0; i < 100; i++ {
		_ = g.AddNode(&Node{ID: fmt.Sprintf("n%d", i), Title: fmt.Sprintf("N%d", i)})
	}
	for i := 0; i < 99; i++ {
		_ = g.AddEdge(&Edge{Source: fmt.Sprintf("n%d", i), Target: fmt.Sprintf("n%d", i+1), Relation: RelRelatedTo})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.MultiHopNeighbors("n0", nil, 5)
	}
}

func BenchmarkHasCycle_NoCycle(b *testing.B) {
	g := NewGraph()
	for i := 0; i < 100; i++ {
		_ = g.AddNode(&Node{ID: fmt.Sprintf("n%d", i), Title: ""})
	}
	for i := 0; i < 99; i++ {
		_ = g.AddEdge(&Edge{Source: fmt.Sprintf("n%d", i), Target: fmt.Sprintf("n%d", i+1), Relation: RelCauses})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.HasCycle()
	}
}

func BenchmarkSubgraph(b *testing.B) {
	g := NewGraph()
	ids := make([]string, 50)
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("n%d", i)
		ids[i] = id
		_ = g.AddNode(&Node{ID: id, Title: id})
	}
	for i := 0; i < 49; i++ {
		_ = g.AddEdge(&Edge{Source: ids[i], Target: ids[i+1], Relation: RelRelatedTo})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Subgraph(ids)
	}
}

func BenchmarkNodeImportance(b *testing.B) {
	g := NewGraph()
	for i := 0; i < 100; i++ {
		_ = g.AddNode(&Node{ID: fmt.Sprintf("n%d", i), Title: ""})
	}
	for i := 0; i < 99; i++ {
		_ = g.AddEdge(&Edge{Source: fmt.Sprintf("n%d", i), Target: fmt.Sprintf("n%d", (i+1)%100), Relation: RelRelatedTo})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.NodeImportance()
	}
}

func BenchmarkSearchNodes(b *testing.B) {
	g := NewGraph()
	for i := 0; i < 200; i++ {
		_ = g.AddNode(&Node{
			ID:      fmt.Sprintf("n%d", i),
			Title:   fmt.Sprintf("Concept about topic %d", i),
			Content: fmt.Sprintf("This discusses technology %d in detail", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.SearchNodes("technology", 20)
	}
}
