package memory

import (
	"encoding/json"
	"errors"
	"os"
)

type localVectorSnapshotV1 struct {
	Version int            `json:"version"`
	Dim     int            `json:"dim"`
	Nodes   []*MemoryNode `json:"nodes"`
}

// WriteSnapshot 将当前向量索引写入 JSON 文件（含维度与各节点向量）
func (s *LocalVectorStore) WriteSnapshot(path string) error {
	if s == nil {
		return errors.New("memory: nil LocalVectorStore")
	}
	s.mu.RLock()
	snap := localVectorSnapshotV1{
		Version: 1,
		Dim:     s.dim,
		Nodes:   make([]*MemoryNode, 0, len(s.nodes)),
	}
	for _, n := range s.nodes {
		if n == nil {
			continue
		}
		nn := *n
		snap.Nodes = append(snap.Nodes, &nn)
	}
	s.mu.RUnlock()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadSnapshot 从 JSON 文件恢复索引（覆盖内存中的 nodes 与 dim）
func (s *LocalVectorStore) ReadSnapshot(path string) error {
	if s == nil {
		return errors.New("memory: nil LocalVectorStore")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snap localVectorSnapshotV1
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dim = snap.Dim
	s.nodes = make(map[string]*MemoryNode)
	for _, n := range snap.Nodes {
		if n == nil || n.MemoryID == "" {
			continue
		}
		nn := *n
		s.nodes[n.MemoryID] = &nn
	}
	return nil
}
