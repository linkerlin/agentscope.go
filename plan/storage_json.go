package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// JSONFileStorage persists plans as individual JSON files in a directory.
type JSONFileStorage struct {
	basePath string
	mu       sync.RWMutex
}

// NewJSONFileStorage creates a JSON file backed plan storage.
func NewJSONFileStorage(basePath string) (*JSONFileStorage, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("plan: create storage dir: %w", err)
	}
	return &JSONFileStorage{basePath: basePath}, nil
}

func (s *JSONFileStorage) filePath(planID string) string {
	safe := strings.ReplaceAll(planID, string(os.PathSeparator), "_")
	return filepath.Join(s.basePath, safe+".json")
}

// AddPlan writes the plan to a JSON file.
func (s *JSONFileStorage) AddPlan(p *Plan) error {
	if p == nil {
		return fmt.Errorf("plan: nil plan")
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.filePath(p.ID), data, 0o644)
}

// GetPlan reads a plan from its JSON file.
func (s *JSONFileStorage) GetPlan(planID string) (*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.filePath(planID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plan not found: %s", planID)
		}
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPlans loads all JSON plan files from the storage directory.
func (s *JSONFileStorage) ListPlans() ([]*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, err
	}
	var out []*Plan
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		planID := strings.TrimSuffix(e.Name(), ".json")
		p, err := s.load(planID)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// DeletePlan removes the plan's JSON file.
func (s *JSONFileStorage) DeletePlan(planID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.filePath(planID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *JSONFileStorage) load(planID string) (*Plan, error) {
	data, err := os.ReadFile(s.filePath(planID))
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
