package embedding

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/model"
)

func cacheKey(model string, input []string) string {
	h := sha256.New()
	h.Write([]byte(model))
	for _, s := range input {
		h.Write([]byte{0})
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func cachePath(dir, key string) string {
	if err := os.MkdirAll(dir, 0755); err != nil {
		// best effort
	}
	return filepath.Join(dir, key+".gob")
}

func saveCache(dir, key string, data []model.EmbeddingData) error {
	path := cachePath(dir, key)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(data)
}

func loadCache(dir, key string) ([]model.EmbeddingData, bool) {
	path := cachePath(dir, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var data []model.EmbeddingData
	if err := gob.NewDecoder(f).Decode(&data); err != nil {
		return nil, false
	}
	return data, true
}
