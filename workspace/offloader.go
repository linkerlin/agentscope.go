package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// Offloader persists compressed context or large tool results to external storage.
type Offloader interface {
	OffloadContext(ctx context.Context, sessionID string, data []byte) (string, error)
	OffloadToolResult(ctx context.Context, sessionID, toolCallID string, data []byte) (string, error)
}

// WorkspaceOffloader stores offloaded payloads inside a Workspace directory.
type WorkspaceOffloader struct {
	WS      Workspace
	BaseDir string
}

func NewWorkspaceOffloader(ws Workspace, baseDir string) *WorkspaceOffloader {
	if baseDir == "" {
		baseDir = ".offload"
	}
	return &WorkspaceOffloader{WS: ws, BaseDir: baseDir}
}

func (o *WorkspaceOffloader) OffloadContext(ctx context.Context, sessionID string, data []byte) (string, error) {
	return o.write(ctx, sessionID, "context", data)
}

func (o *WorkspaceOffloader) OffloadToolResult(ctx context.Context, sessionID, toolCallID string, data []byte) (string, error) {
	name := fmt.Sprintf("tool_%s", toolCallID)
	return o.write(ctx, sessionID, name, data)
}

func (o *WorkspaceOffloader) write(ctx context.Context, sessionID, kind string, data []byte) (string, error) {
	if o.WS == nil {
		return "", fmt.Errorf("offloader: nil workspace")
	}
	ref := filepath.Join(o.BaseDir, sessionID, fmt.Sprintf("%s_%d.json", kind, time.Now().UnixNano()))
	wrapper, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"kind":       kind,
		"data":       json.RawMessage(data),
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err := o.WS.WriteFile(ctx, ref, wrapper, 0644); err != nil {
		return "", err
	}
	return ref, nil
}

// MemoryOffloader keeps offloaded payloads in memory (for tests/dev).
type MemoryOffloader struct {
	store map[string][]byte
}

func NewMemoryOffloader() *MemoryOffloader {
	return &MemoryOffloader{store: make(map[string][]byte)}
}

func (o *MemoryOffloader) OffloadContext(_ context.Context, sessionID string, data []byte) (string, error) {
	ref := fmt.Sprintf("mem://%s/context", sessionID)
	o.store[ref] = append([]byte(nil), data...)
	return ref, nil
}

func (o *MemoryOffloader) OffloadToolResult(_ context.Context, sessionID, toolCallID string, data []byte) (string, error) {
	ref := fmt.Sprintf("mem://%s/tool/%s", sessionID, toolCallID)
	o.store[ref] = append([]byte(nil), data...)
	return ref, nil
}

func (o *MemoryOffloader) Get(ref string) ([]byte, bool) {
	b, ok := o.store[ref]
	return b, ok
}
