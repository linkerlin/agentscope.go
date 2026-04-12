package memory

import "strings"

// HybridScore 结合向量相似度与简易词面重合（BM25 的极简替代：词袋重叠率）
func HybridScore(vectorSim float64, query, doc string, vectorWeight float64) float64 {
	if vectorWeight < 0 {
		vectorWeight = 0
	}
	if vectorWeight > 1 {
		vectorWeight = 1
	}
	tok := tokenize(query)
	overlap := overlapRatio(tok, tokenize(doc))
	lex := (overlap + 1e-6) / (1.0 + 1e-6)
	return vectorWeight*vectorSim + (1-vectorWeight)*lex
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	for _, sep := range []string{"\n", "\t", ",", ".", "!", "?", ";", ":"} {
		s = strings.ReplaceAll(s, sep, " ")
	}
	fields := strings.Fields(s)
	return fields
}

func overlapRatio(qtok, dtok []string) float64 {
	if len(qtok) == 0 || len(dtok) == 0 {
		return 0
	}
	dset := make(map[string]int)
	for _, t := range dtok {
		dset[t]++
	}
	var hit int
	for _, t := range qtok {
		if dset[t] > 0 {
			hit++
		}
	}
	return float64(hit) / float64(len(qtok))
}

// RankMemoryNodesHybrid 对已有向量检索结果按 HybridScore 重排
func RankMemoryNodesHybrid(nodes []*MemoryNode, query string, vectorWeight float64) []*MemoryNode {
	if len(nodes) == 0 {
		return nodes
	}
	type scored struct {
		n *MemoryNode
		f float64
	}
	var out []scored
	for _, n := range nodes {
		vs := n.Score
		if vs > 1 {
			vs = 1
		}
		if vs < 0 {
			vs = 0
		}
		h := HybridScore(vs, query, n.Content, vectorWeight)
		nn := *n
		nn.Score = h
		out = append(out, scored{&nn, h})
	}
	// 排序
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].f > out[i].f {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	res := make([]*MemoryNode, len(out))
	for i := range out {
		res[i] = out[i].n
	}
	return res
}
