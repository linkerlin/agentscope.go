package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// TextLoader loads plain-text documents from the local filesystem.
type TextLoader struct {
	// Encoding defaults to UTF-8 if empty.
	Encoding string
}

// Load reads a single file and returns it as a Document.
func (l *TextLoader) Load(ctx context.Context, source string) ([]Document, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("text loader: %w", err)
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("text loader: %w", err)
	}
	return []Document{{
		Content: string(data),
		Metadata: map[string]any{
			"source":   source,
			"filename": filepath.Base(source),
			"size":     info.Size(),
			"mod_time": info.ModTime(),
		},
	}}, nil
}

var _ Loader = (*TextLoader)(nil)
