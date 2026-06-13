package memory

import "testing"

func TestHybridScore(t *testing.T) {
	h := HybridScore(0.9, "hello world", "hello there", 0.5)
	if h < 0 || h > 1.01 {
		t.Fatalf("hybrid %f", h)
	}
}

func TestRankMemoryNodesHybrid(t *testing.T) {
	nodes := []*MemoryNode{
		{Content: "foo bar", Score: 0.8},
		{Content: "baz", Score: 0.9},
	}
	out := RankMemoryNodesHybrid(nodes, "foo bar", 0.5, nil)
	if len(out) != 2 {
		t.Fatal(len(out))
	}
}
