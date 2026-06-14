package graph

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// --- Graph Construction Helpers ---

func buildTestGraph() *Graph {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a", Title: "Alpha"})
	_ = g.AddNode(&Node{ID: "b", Title: "Beta"})
	_ = g.AddNode(&Node{ID: "c", Title: "Gamma"})
	_ = g.AddNode(&Node{ID: "d", Title: "Delta"})
	_ = g.AddNode(&Node{ID: "e", Title: "Epsilon"})
	return g
}

func buildChainGraph() *Graph {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a", Title: "A"})
	_ = g.AddNode(&Node{ID: "b", Title: "B"})
	_ = g.AddNode(&Node{ID: "c", Title: "C"})
	_ = g.AddNode(&Node{ID: "d", Title: "D"})
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelCauses})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Relation: RelCauses})
	_ = g.AddEdge(&Edge{Source: "c", Target: "d", Relation: RelCauses})
	return g
}

// --- Basic CRUD Tests ---

func TestAddAndGetNode(t *testing.T) {
	g := NewGraph()
	n := &Node{ID: "x", Title: "Test", Aliases: []string{"t", "test"}}
	if err := g.AddNode(n); err != nil {
		t.Fatal(err)
	}
	if g.GetNode("x") == nil {
		t.Fatal("expected node x")
	}
	if got := g.GetNodeByAlias("test"); got == nil || got.ID != "x" {
		t.Errorf("alias lookup failed: %v", got)
	}
	if got := g.GetNodeByAlias("TEST"); got == nil {
		t.Error("alias should be case-insensitive")
	}
}

func TestAddNode_Invalid(t *testing.T) {
	g := NewGraph()
	if err := g.AddNode(nil); err == nil {
		t.Error("expected error for nil node")
	}
	if err := g.AddNode(&Node{}); err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestAddEdge(t *testing.T) {
	g := buildTestGraph()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo, Bidirectional: true})
	edges := g.GetEdges("a")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge from a, got %d", len(edges))
	}
	edgesB := g.GetEdges("b")
	if len(edgesB) != 1 {
		t.Fatalf("expected 1 reverse edge from b (bidirectional), got %d", len(edgesB))
	}
}

func TestGetNeighbors(t *testing.T) {
	g := buildTestGraph()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "c", Target: "a", Relation: RelSupports})
	neighbors := g.GetNeighbors("a")
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d: %v", len(neighbors), neighbors)
	}
}

func TestCount(t *testing.T) {
	g := buildTestGraph()
	if g.Count() != 5 {
		t.Errorf("expected 5 nodes, got %d", g.Count())
	}
}

func TestDeleteNode(t *testing.T) {
	g := buildTestGraph()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	g.DeleteNode("a")
	if g.GetNode("a") != nil {
		t.Error("node a should be deleted")
	}
	if g.GetEdges("a") != nil && len(g.GetEdges("a")) > 0 {
		t.Error("edges from a should be deleted")
	}
	edgesB := g.GetEdges("b")
	for _, e := range edgesB {
		if e.Target == "a" {
			t.Error("edges pointing to a should be cleaned")
		}
	}
}

func TestExpand(t *testing.T) {
	g := buildTestGraph()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Relation: RelRelatedTo})
	nodes := g.Expand("a", 3)
	if len(nodes) < 3 {
		t.Errorf("expected at least 3 nodes in 3-hop expansion, got %d", len(nodes))
	}
}

func TestFindPath(t *testing.T) {
	g := buildTestGraph()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Relation: RelRelatedTo})
	path := g.FindPath("a", "c", 5)
	if len(path) != 3 || path[0] != "a" || path[2] != "c" {
		t.Errorf("unexpected path: %v", path)
	}
}

func TestFindPath_NotFound(t *testing.T) {
	g := buildTestGraph()
	path := g.FindPath("a", "e", 1)
	if path != nil {
		t.Errorf("expected nil path for unreachable, got %v", path)
	}
}

func TestExtractWikilinks(t *testing.T) {
	text := "See [[Go]] and [[Python|py]] for details."
	links := ExtractWikilinks(text)
	if len(links) != 2 {
		t.Fatalf("expected 2 wikilinks, got %d", len(links))
	}
	if links[0].Concept != "Go" {
		t.Errorf("expected 'Go', got '%s'", links[0].Concept)
	}
	if links[1].Concept != "Python" || links[1].Alias != "py" {
		t.Errorf("expected Python|py, got %s|%s", links[1].Concept, links[1].Alias)
	}
}

// --- Reasoning Algorithm Tests ---

func TestFindAllPaths_LinearChain(t *testing.T) {
	g := buildChainGraph()
	paths := g.FindAllPaths("a", "d", 5, 10)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if len(paths[0]) != 4 {
		t.Errorf("expected path length 4 (a->b->c->d), got %d", len(paths[0]))
	}
}

func TestFindAllPaths_MultiplePaths(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		_ = g.AddNode(&Node{ID: id, Title: id})
	}
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "a", Target: "c", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "b", Target: "d", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "c", Target: "d", Relation: RelRelatedTo})
	paths := g.FindAllPaths("a", "d", 5, 10)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths (a->b->d, a->c->d), got %d", len(paths))
	}
}

func TestFindAllPaths_MaxPathsLimit(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		_ = g.AddNode(&Node{ID: id, Title: id})
	}
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "a", Target: "c", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "b", Target: "d", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "c", Target: "d", Relation: RelRelatedTo})
	paths := g.FindAllPaths("a", "d", 5, 1)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path (limited), got %d", len(paths))
	}
}

func TestFindAllPaths_SameNode(t *testing.T) {
	g := buildTestGraph()
	paths := g.FindAllPaths("a", "a", 3, 5)
	if len(paths) != 1 || len(paths[0]) != 1 {
		t.Errorf("expected single trivial path, got %v", paths)
	}
}

func TestMultiHopNeighbors_AllRelations(t *testing.T) {
	g := buildChainGraph()
	neighbors := g.MultiHopNeighbors("a", nil, 3)
	if len(neighbors) != 3 {
		t.Fatalf("expected 3 reachable (b, c, d), got %d: %v", len(neighbors), neighbors)
	}
}

func TestMultiHopNeighbors_FilteredByRelation(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a", Title: "A"})
	_ = g.AddNode(&Node{ID: "b", Title: "B"})
	_ = g.AddNode(&Node{ID: "c", Title: "C"})
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelCauses})
	_ = g.AddEdge(&Edge{Source: "a", Target: "c", Relation: RelMentions})
	neighbors := g.MultiHopNeighbors("a", []Relation{RelCauses}, 1)
	if len(neighbors) != 1 || neighbors[0] != "b" {
		t.Errorf("expected only 'b' via causes, got %v", neighbors)
	}
}

func TestMultiHopNeighbors_LimitedHops(t *testing.T) {
	g := buildChainGraph()
	neighbors := g.MultiHopNeighbors("a", nil, 1)
	if len(neighbors) != 1 || neighbors[0] != "b" {
		t.Errorf("expected only 'b' within 1 hop, got %v", neighbors)
	}
}

func TestRelatedNodes(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a", Title: "A"})
	_ = g.AddNode(&Node{ID: "b", Title: "B"})
	_ = g.AddNode(&Node{ID: "c", Title: "C"})
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelSupports})
	_ = g.AddEdge(&Edge{Source: "a", Target: "c", Relation: RelContradicts})
	related := g.RelatedNodes("a", []Relation{RelSupports})
	if len(related) != 1 || related[0].ID != "b" {
		t.Errorf("expected node 'b', got %v", related)
	}
}

func TestSubgraph(t *testing.T) {
	g := buildChainGraph()
	sub := g.Subgraph([]string{"a", "b", "d"})
	if sub.Count() != 3 {
		t.Fatalf("expected 3 nodes in subgraph, got %d", sub.Count())
	}
	if len(sub.GetEdges("a")) != 1 {
		t.Errorf("expected 1 edge a->b in subgraph, got %d", len(sub.GetEdges("a")))
	}
	if len(sub.GetEdges("b")) != 0 {
		t.Errorf("expected 0 edges from b in subgraph (c excluded), got %d", len(sub.GetEdges("b")))
	}
}

func TestHasCycle_NoCycle(t *testing.T) {
	g := buildChainGraph()
	if g.HasCycle() {
		t.Error("chain graph should not have a cycle")
	}
}

func TestHasCycle_WithCycle(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a", Title: "A"})
	_ = g.AddNode(&Node{ID: "b", Title: "B"})
	_ = g.AddNode(&Node{ID: "c", Title: "C"})
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelCauses})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Relation: RelCauses})
	_ = g.AddEdge(&Edge{Source: "c", Target: "a", Relation: RelCauses})
	if !g.HasCycle() {
		t.Error("expected cycle in a->b->c->a")
	}
}

func TestNodeImportance(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "hub", Title: "Hub"})
	_ = g.AddNode(&Node{ID: "spoke1", Title: "S1"})
	_ = g.AddNode(&Node{ID: "spoke2", Title: "S2"})
	_ = g.AddNode(&Node{ID: "spoke3", Title: "S3"})
	_ = g.AddEdge(&Edge{Source: "hub", Target: "spoke1", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "hub", Target: "spoke2", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "hub", Target: "spoke3", Relation: RelRelatedTo})
	imp := g.NodeImportance()
	if imp["hub"] != 3 {
		t.Errorf("expected hub importance 3, got %f", imp["hub"])
	}
	if imp["spoke1"] != 1 {
		t.Errorf("expected spoke1 importance 1, got %f", imp["spoke1"])
	}
}

func TestSearchNodes(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "go", Title: "Go Language", Content: "A programming language"})
	_ = g.AddNode(&Node{ID: "py", Title: "Python", Content: "Another programming language"})
	_ = g.AddNode(&Node{ID: "rust", Title: "Rust", Content: "Systems programming"})
	results := g.SearchNodes("language", 5)
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'language', got %d", len(results))
	}
}

func TestAllNodeIDs(t *testing.T) {
	g := buildTestGraph()
	ids := g.AllNodeIDs()
	if len(ids) != 5 {
		t.Fatalf("expected 5 IDs, got %d", len(ids))
	}
}

func TestAllEdges(t *testing.T) {
	g := buildTestGraph()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Relation: RelRelatedTo})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Relation: RelCauses})
	edges := g.AllEdges()
	if len(edges) < 2 {
		t.Errorf("expected at least 2 edges, got %d", len(edges))
	}
}

// --- Knowledge Extractor Tests ---

type mockExtractModel struct {
	response string
}

func (m *mockExtractModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.response).Build(), nil
}

func (m *mockExtractModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func (m *mockExtractModel) ModelName() string { return "mock-extract" }

func TestKnowledgeExtractor_Extract(t *testing.T) {
	m := &mockExtractModel{
		response: `{"entities":[{"name":"Go","type":"concept","description":"A programming language"},{"name":"Docker","type":"concept","description":"Container platform"}],"triples":[{"subject":"Go","relation":"supports","object":"Docker"}]}`,
	}
	extractor := NewKnowledgeExtractor(m)

	triples, entities, err := extractor.Extract(context.Background(), "Go supports Docker containers.")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "Go" || triples[0].Object != "Docker" {
		t.Errorf("unexpected triple: %+v", triples[0])
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
}

func TestKnowledgeExtractor_ExtractAndAdd(t *testing.T) {
	m := &mockExtractModel{
		response: `{"entities":[{"name":"Go","type":"concept"},{"name":"Docker","type":"concept"},{"name":"Kubernetes","type":"concept"}],"triples":[{"subject":"Go","relation":"supports","object":"Docker"},{"subject":"Docker","relation":"part_of","object":"Kubernetes"}]}`,
	}
	extractor := NewKnowledgeExtractor(m)
	g := NewGraph()

	created, err := extractor.ExtractAndAdd(context.Background(), "Go supports Docker, which is part of Kubernetes.", g)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("expected 3 nodes created, got %d", len(created))
	}
	if g.Count() != 3 {
		t.Fatalf("expected 3 nodes in graph, got %d", g.Count())
	}

	related := g.RelatedNodes("go", []Relation{RelSupports})
	if len(related) != 1 || related[0].ID != "docker" {
		t.Errorf("expected go --supports--> docker, got %v", related)
	}

	parts := g.RelatedNodes("docker", []Relation{RelPartOf})
	if len(parts) != 1 || parts[0].ID != "kubernetes" {
		t.Errorf("expected docker --part_of--> kubernetes, got %v", parts)
	}
}

func TestKnowledgeExtractor_EmptyText(t *testing.T) {
	extractor := NewKnowledgeExtractor(&mockExtractModel{response: "{}"})
	triples, entities, err := extractor.Extract(context.Background(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 0 || len(entities) != 0 {
		t.Error("expected empty results for empty text")
	}
}

func TestKnowledgeExtractor_StripsMarkdownFence(t *testing.T) {
	m := &mockExtractModel{
		response: "```json\n{\"entities\":[],\"triples\":[{\"subject\":\"A\",\"relation\":\"causes\",\"object\":\"B\"}]}\n```",
	}
	extractor := NewKnowledgeExtractor(m)
	triples, _, err := extractor.Extract(context.Background(), "A causes B")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple after stripping fence, got %d", len(triples))
	}
}

func TestKnowledgeExtractor_NoModel(t *testing.T) {
	extractor := NewKnowledgeExtractor(nil)
	_, _, err := extractor.Extract(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for nil model")
	}
}

func TestNormalizeID(t *testing.T) {
	if got := normalizeID("Go Language"); got != "go_language" {
		t.Errorf("expected 'go_language', got '%s'", got)
	}
	if got := normalizeID("Test-Case"); got != "test_case" {
		t.Errorf("expected 'test_case', got '%s'", got)
	}
}

func TestStripMarkdownFence(t *testing.T) {
	if got := stripMarkdownFence("```json\n{}\n```"); got != "{}" {
		t.Errorf("unexpected: '%s'", got)
	}
	if got := stripMarkdownFence("plain text"); got != "plain text" {
		t.Errorf("unexpected: '%s'", got)
	}
}
