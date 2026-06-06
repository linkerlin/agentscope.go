package loader

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
)

// DirLoader recursively loads all files under a directory that pass the filter.
type DirLoader struct {
	// Loader is used to load each individual file. Defaults to TextLoader.
	Loader Loader
	// Filter decides whether a file should be loaded.
	// If nil, all regular files are accepted.
	Filter func(path string, info fs.FileInfo) bool
}

// Load walks the directory and loads matching files.
func (d *DirLoader) Load(ctx context.Context, source string) ([]Document, error) {
	loader := d.Loader
	if loader == nil {
		loader = &TextLoader{}
	}

	var docs []Document
	err := filepath.Walk(source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if d.Filter != nil && !d.Filter(path, info) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		loaded, err := loader.Load(ctx, path)
		if err != nil {
			return fmt.Errorf("dir loader: %w", err)
		}
		docs = append(docs, loaded...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

var _ Loader = (*DirLoader)(nil)
