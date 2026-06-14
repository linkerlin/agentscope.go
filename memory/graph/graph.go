package graph

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Node 知识图谱节点
type Node struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Type     NodeType       `json:"type"`
	Aliases  []string       `json:"aliases"`
	Outlinks []string       `json:"outlinks"` // 指向的节点 ID
	Inlinks  []string       `json:"inlinks"`  // 被指向的节点 ID
	Metadata map[string]any `json:"metadata"`
}

// NodeType 节点类型
type NodeType string

const (
	NodeTypeConcept NodeType = "concept" // 概念节点
	NodeTypeMemory  NodeType = "memory"  // 记忆节点
	NodeTypePerson  NodeType = "person"  // 人物节点
	NodeTypeEvent   NodeType = "event"   // 事件节点
	NodeTypeSource  NodeType = "source"  // 来源节点
)

// Edge 知识图谱边
type Edge struct {
	Source        string   `json:"source"`        // 源节点 ID
	Target        string   `json:"target"`        // 目标节点 ID
	Relation      Relation `json:"relation"`      // 关系类型
	Weight        float64  `json:"weight"`        // 权重 0-1
	Bidirectional bool     `json:"bidirectional"` // 是否双向
}

// Relation 关系类型
type Relation string

const (
	RelRelatedTo   Relation = "relates_to"   // 相关
	RelDerivedFrom Relation = "derived_from" // 派生自
	RelPartOf      Relation = "part_of"      // 属于
	RelCauses      Relation = "causes"       // 导致
	RelContradicts Relation = "contradicts"  // 矛盾
	RelSupports    Relation = "supports"     // 支持
	RelMentions    Relation = "mentions"     // 提及
)

// Graph 知识图谱
type Graph struct {
	mu      sync.RWMutex
	nodes   map[string]*Node
	edges   map[string][]*Edge // source -> edges
	aliases map[string]string  // alias -> nodeID
}

// NewGraph 创建知识图谱
func NewGraph() *Graph {
	return &Graph{
		nodes:   make(map[string]*Node),
		edges:   make(map[string][]*Edge),
		aliases: make(map[string]string),
	}
}

// AddNode 添加节点
func (g *Graph) AddNode(node *Node) error {
	if g == nil || node == nil || node.ID == "" {
		return fmt.Errorf("invalid node")
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node
	// 注册别名
	for _, alias := range node.Aliases {
		if alias != "" {
			g.aliases[strings.ToLower(alias)] = node.ID
		}
	}
	return nil
}

// GetNode 获取节点
func (g *Graph) GetNode(id string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// GetNodeByAlias 通过别名获取节点
func (g *Graph) GetNodeByAlias(alias string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	id, ok := g.aliases[strings.ToLower(alias)]
	if !ok {
		return nil
	}
	return g.nodes[id]
}

// AddEdge 添加边（自动双向）
func (g *Graph) AddEdge(edge *Edge) error {
	if g == nil || edge == nil || edge.Source == "" || edge.Target == "" {
		return fmt.Errorf("invalid edge")
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	// 添加正向边
	g.edges[edge.Source] = append(g.edges[edge.Source], edge)

	// 更新节点的 outlinks 和 inlinks
	if src, ok := g.nodes[edge.Source]; ok {
		src.Outlinks = appendUnique(src.Outlinks, edge.Target)
	}
	if tgt, ok := g.nodes[edge.Target]; ok {
		tgt.Inlinks = appendUnique(tgt.Inlinks, edge.Source)
	}

	// 如果双向，添加反向边
	if edge.Bidirectional {
		reverse := &Edge{
			Source:        edge.Target,
			Target:        edge.Source,
			Relation:      edge.Relation,
			Weight:        edge.Weight,
			Bidirectional: true,
		}
		g.edges[edge.Target] = append(g.edges[edge.Target], reverse)
	}
	return nil
}

// GetEdges 获取节点的所有边
func (g *Graph) GetEdges(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edges[nodeID]
}

// GetNeighbors 获取节点的邻居（outlinks + inlinks）
func (g *Graph) GetNeighbors(nodeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[nodeID]
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var neighbors []string
	for _, id := range node.Outlinks {
		if !seen[id] {
			seen[id] = true
			neighbors = append(neighbors, id)
		}
	}
	for _, id := range node.Inlinks {
		if !seen[id] {
			seen[id] = true
			neighbors = append(neighbors, id)
		}
	}
	return neighbors
}

// Expand 渐进式展开：从中心节点展开 N 层
func (g *Graph) Expand(centerID string, depth int) []*Node {
	if depth <= 0 {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	seen := make(map[string]bool)
	var result []*Node
	queue := []string{centerID}
	seen[centerID] = true

	for d := 0; d < depth && len(queue) > 0; d++ {
		nextQueue := []string{}
		for _, id := range queue {
			node, ok := g.nodes[id]
			if !ok {
				continue
			}
			result = append(result, node)
			for _, nid := range node.Outlinks {
				if !seen[nid] {
					seen[nid] = true
					nextQueue = append(nextQueue, nid)
				}
			}
			for _, nid := range node.Inlinks {
				if !seen[nid] {
					seen[nid] = true
					nextQueue = append(nextQueue, nid)
				}
			}
		}
		queue = nextQueue
	}
	return result
}

// FindPath 查找两个节点之间的路径（BFS）
func (g *Graph) FindPath(from, to string, maxDepth int) []string {
	if from == to {
		return []string{from}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]string) // node -> parent
	queue := []string{from}
	visited[from] = ""

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		next := []string{}
		for _, id := range queue {
			node, ok := g.nodes[id]
			if !ok {
				continue
			}
			for _, nid := range append(node.Outlinks, node.Inlinks...) {
				if _, seen := visited[nid]; seen {
					continue
				}
				visited[nid] = id
				if nid == to {
					// 重建路径
					path := []string{nid}
					for cur := nid; cur != ""; cur = visited[cur] {
						if cur != nid {
							path = append([]string{cur}, path...)
						}
					}
					return path
				}
				next = append(next, nid)
			}
		}
		queue = next
	}
	return nil
}

// DeleteNode 删除节点及其边
func (g *Graph) DeleteNode(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, ok := g.nodes[id]
	if !ok {
		return
	}

	// 删除相关边
	delete(g.edges, id)
	for k, edges := range g.edges {
		filtered := edges[:0]
		for _, e := range edges {
			if e.Target != id && e.Source != id {
				filtered = append(filtered, e)
			}
		}
		g.edges[k] = filtered
	}

	// 删除别名
	for _, alias := range node.Aliases {
		delete(g.aliases, strings.ToLower(alias))
	}
	delete(g.nodes, id)
}

// Count 返回节点数
func (g *Graph) Count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// --- Wikilink 解析器 ---

// wikilinkRegex 匹配 [[概念]] 和 [[概念|别名]]
var wikilinkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

// ExtractWikilinks 从文本中提取 Wikilink
func ExtractWikilinks(text string) []Wikilink {
	var links []Wikilink
	matches := wikilinkRegex.FindAllStringSubmatchIndex(text, -1)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		// m[0]: [[...]] 起始, m[1]: 结束
		// m[2]: 概念 起始, m[3]: 概念 结束
		// m[4]: 别名 起始, m[5]: 别名 结束
		concept := text[m[2]:m[3]]
		link := Wikilink{
			Raw:     text[m[0]:m[1]],
			Concept: strings.TrimSpace(concept),
			Start:   m[0],
			End:     m[1],
		}
		if m[4] >= 0 && m[5] >= 0 {
			link.Alias = strings.TrimSpace(text[m[4]:m[5]])
		}
		links = append(links, link)
	}
	return links
}

// Wikilink Wikilink 结构
type Wikilink struct {
	Raw     string // 原始文本 [[...]]
	Concept string // 概念名
	Alias   string // 别名（可选）
	Start   int    // 起始位置
	End     int    // 结束位置
}

// --- 辅助函数 ---

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
