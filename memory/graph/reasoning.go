package graph

import (
	"sort"
	"strings"
)

// AllNodeIDs returns all node IDs in the graph.
func (g *Graph) AllNodeIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// AllEdges returns all edges in the graph.
func (g *Graph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var edges []*Edge
	for _, el := range g.edges {
		edges = append(edges, el...)
	}
	return edges
}

// FindAllPaths finds all simple paths (no repeated nodes) from source to target
// within maxDepth hops. Returns at most maxPaths paths to avoid explosion.
func (g *Graph) FindAllPaths(from, to string, maxDepth, maxPaths int) [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if from == to {
		return [][]string{{from}}
	}
	if g.nodes[from] == nil || g.nodes[to] == nil {
		return nil
	}

	var results [][]string
	visited := map[string]bool{from: true}

	var dfs func(current string, path []string)
	dfs = func(current string, path []string) {
		if len(results) >= maxPaths {
			return
		}
		if current == to {
			cp := make([]string, len(path))
			copy(cp, path)
			results = append(results, cp)
			return
		}
		if len(path)-1 >= maxDepth {
			return
		}
		node := g.nodes[current]
		if node == nil {
			return
		}
		for _, nid := range node.Outlinks {
			if !visited[nid] {
				visited[nid] = true
				dfs(nid, append(path, nid))
				delete(visited, nid)
			}
		}
		for _, nid := range node.Inlinks {
			if !visited[nid] {
				visited[nid] = true
				dfs(nid, append(path, nid))
				delete(visited, nid)
			}
		}
	}

	dfs(from, []string{from})
	return results
}

// MultiHopNeighbors returns all node IDs reachable within maxHops from the
// starting node, optionally filtered by relation types. If relations is empty,
// all edge types are followed.
func (g *Graph) MultiHopNeighbors(nodeID string, relations []Relation, maxHops int) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.nodes[nodeID] == nil || maxHops <= 0 {
		return nil
	}

	relFilter := make(map[Relation]bool)
	for _, r := range relations {
		relFilter[r] = true
	}

	visited := map[string]bool{nodeID: true}
	queue := []string{nodeID}

	for hop := 0; hop < maxHops; hop++ {
		var nextQueue []string
		for _, id := range queue {
			neighbors := g.getFilteredNeighbors(id, relFilter)
			for _, nid := range neighbors {
				if !visited[nid] {
					visited[nid] = true
					nextQueue = append(nextQueue, nid)
				}
			}
		}
		queue = nextQueue
		if len(queue) == 0 {
			break
		}
	}

	result := make([]string, 0, len(visited)-1)
	for id := range visited {
		if id != nodeID {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func (g *Graph) getFilteredNeighbors(nodeID string, relFilter map[Relation]bool) []string {
	var neighbors []string
	for _, edge := range g.edges[nodeID] {
		if len(relFilter) > 0 && !relFilter[edge.Relation] {
			continue
		}
		if edge.Target != nodeID {
			neighbors = append(neighbors, edge.Target)
		}
	}
	for id, edges := range g.edges {
		if id == nodeID {
			continue
		}
		for _, edge := range edges {
			if edge.Target != nodeID {
				continue
			}
			if len(relFilter) > 0 && !relFilter[edge.Relation] {
				continue
			}
			neighbors = append(neighbors, edge.Source)
		}
	}
	return neighbors
}

// RelatedNodes returns nodes connected to the given node via the specified
// relation types. If relations is empty, all neighbors are returned.
func (g *Graph) RelatedNodes(nodeID string, relations []Relation) []*Node {
	ids := g.MultiHopNeighbors(nodeID, relations, 1)
	g.mu.RLock()
	defer g.mu.RUnlock()
	var nodes []*Node
	for _, id := range ids {
		if n := g.nodes[id]; n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// Subgraph extracts a subgraph containing only the specified node IDs and
// the edges between them.
func (g *Graph) Subgraph(nodeIDs []string) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	idSet := make(map[string]bool)
	for _, id := range nodeIDs {
		idSet[id] = true
	}

	sub := NewGraph()
	for _, id := range nodeIDs {
		if n := g.nodes[id]; n != nil {
			cp := *n
			cp.Outlinks = nil
			cp.Inlinks = nil
			sub.nodes[id] = &cp
			for _, alias := range n.Aliases {
				if alias != "" {
					sub.aliases[strings.ToLower(alias)] = id
				}
			}
		}
	}

	for _, id := range nodeIDs {
		for _, edge := range g.edges[id] {
			if idSet[edge.Target] {
				sub.edges[edge.Source] = append(sub.edges[edge.Source], edge)
				if src := sub.nodes[edge.Source]; src != nil {
					src.Outlinks = appendUnique(src.Outlinks, edge.Target)
				}
				if tgt := sub.nodes[edge.Target]; tgt != nil {
					tgt.Inlinks = appendUnique(tgt.Inlinks, edge.Source)
				}
			}
		}
	}

	return sub
}

// HasCycle checks whether the graph contains a directed cycle.
// Only follows Outlinks (directed edges) for cycle detection.
func (g *Graph) HasCycle() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)

	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		node := g.nodes[id]
		if node != nil {
			for _, out := range node.Outlinks {
				if color[out] == gray {
					return true
				}
				if color[out] == white && visit(out) {
					return true
				}
			}
		}
		color[id] = black
		return false
	}

	for id := range g.nodes {
		if color[id] == white {
			if visit(id) {
				return true
			}
		}
	}
	return false
}

// NodeImportance calculates degree-based centrality for all nodes.
// The score is the total number of connections (inlinks + outlinks).
func (g *Graph) NodeImportance() map[string]float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]float64, len(g.nodes))
	for id, node := range g.nodes {
		seen := make(map[string]bool)
		for _, out := range node.Outlinks {
			seen[out] = true
		}
		for _, in := range node.Inlinks {
			seen[in] = true
		}
		result[id] = float64(len(seen))
	}
	return result
}

// SearchNodes returns node IDs whose Title or Content contains the query string
// (case-insensitive). Returns at most maxResults IDs.
func (g *Graph) SearchNodes(query string, maxResults int) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	query = strings.ToLower(query)
	var results []*Node
	for _, node := range g.nodes {
		if strings.Contains(strings.ToLower(node.Title), query) ||
			strings.Contains(strings.ToLower(node.Content), query) {
			results = append(results, node)
			if maxResults > 0 && len(results) >= maxResults {
				break
			}
		}
	}
	return results
}
