// Package blob stores uploaded document bytes between the upload endpoint and
// the indexing worker. Backends: Local (filesystem) now; S3 later.
//
// Aligned with Python AgentScope app.rag.blob_store (BlobStoreBase).
package blob

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BlobStore stores and retrieves raw document bytes by key.
type BlobStore interface {
	// Put stores the content read from r under key and returns a URI that Get
	// can later resolve. The URI scheme identifies the backend.
	Put(ctx context.Context, key string, r io.Reader) (uri string, err error)
	// Get opens the stored blob for reading. The caller must Close it.
	Get(ctx context.Context, uri string) (io.ReadCloser, error)
	// Delete removes the stored blob. Missing blobs are not an error.
	Delete(ctx context.Context, uri string) error
}

// LocalBlobStore persists blobs on the local filesystem under RootDir.
// URIs use the "local://" scheme: local://<key>.
type LocalBlobStore struct {
	RootDir string
}

// NewLocalBlobStore creates a LocalBlobStore rooted at rootDir, creating it if
// necessary.
func NewLocalBlobStore(rootDir string) (*LocalBlobStore, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("blob: cannot create root dir: %w", err)
	}
	return &LocalBlobStore{RootDir: rootDir}, nil
}

func (s *LocalBlobStore) keyPath(key string) string {
	return filepath.Join(s.RootDir, key)
}

// Put writes the content to <RootDir>/<key> and returns local://<key>.
func (s *LocalBlobStore) Put(ctx context.Context, key string, r io.Reader) (string, error) {
	path := s.keyPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return "local://" + key, nil
}

// Get opens local://<key> for reading.
func (s *LocalBlobStore) Get(ctx context.Context, uri string) (io.ReadCloser, error) {
	key, ok := parseLocalURI(uri)
	if !ok {
		return nil, fmt.Errorf("blob: unsupported uri %q", uri)
	}
	return os.Open(s.keyPath(key))
}

// Delete removes the blob; a missing file is not an error.
func (s *LocalBlobStore) Delete(ctx context.Context, uri string) error {
	key, ok := parseLocalURI(uri)
	if !ok {
		return fmt.Errorf("blob: unsupported uri %q", uri)
	}
	if err := os.Remove(s.keyPath(key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func parseLocalURI(uri string) (string, bool) {
	const prefix = "local://"
	if len(uri) > len(prefix) && uri[:len(prefix)] == prefix {
		return uri[len(prefix):], true
	}
	return "", false
}
