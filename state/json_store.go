package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// JSONStore 将状态以 JSON 文件形式持久化到目录（每键一个文件：key.json）
type JSONStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewJSONStore 创建 JSON 文件存储，目录不存在时会创建
func NewJSONStore(basePath string) (*JSONStore, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, err
	}
	return &JSONStore{basePath: basePath}, nil
}

func (s *JSONStore) filePath(key string) string {
	safe := strings.ReplaceAll(key, string(os.PathSeparator), "_")
	return filepath.Join(s.basePath, safe+".json")
}

// Save 将 State 序列化为 JSON 写入磁盘
func (s *JSONStore) Save(key string, value State) error {
	if key == "" {
		return errors.New("state: empty key")
	}
	if value == nil {
		return errors.New("state: nil value")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.filePath(key), data, 0o644)
}

// Get 从磁盘读取并反序列化到 dest（dest 须为指向具体类型的指针，且实现 State）
func (s *JSONStore) Get(key string, dest State) error {
	if key == "" {
		return errors.New("state: empty key")
	}
	if dest == nil {
		return errors.New("state: nil dest")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.filePath(key))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Exists 判断键是否存在
func (s *JSONStore) Exists(key string) bool {
	if key == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(s.filePath(key))
	return err == nil
}

// Delete 删除键对应文件
func (s *JSONStore) Delete(key string) error {
	if key == "" {
		return errors.New("state: empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.filePath(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListKeys 列出目录下全部 .json 键名（不含后缀）
func (s *JSONStore) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil
	}
	var keys []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			keys = append(keys, strings.TrimSuffix(name, ".json"))
		}
	}
	return keys
}
