package memory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileChangeCallback 文件变更回调
type FileChangeCallback func(changedFiles []string)

// FileWatcher 监控 MEMORY.md 和 memory/ 目录变更，自动触发回调。
type FileWatcher struct {
	dir        string
	interval   time.Duration
	onChange   FileChangeCallback
	lastHashes map[string]string
	mu         sync.Mutex
	stopCh     chan struct{}
	started    bool
}

// NewFileWatcher 创建文件监控器
func NewFileWatcher(dir string, interval time.Duration) *FileWatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &FileWatcher{
		dir:        dir,
		interval:   interval,
		lastHashes: make(map[string]string),
		stopCh:     make(chan struct{}),
	}
}

// OnChange 设置变更回调
func (w *FileWatcher) OnChange(callback FileChangeCallback) {
	w.onChange = callback
}

// Start 开始监控
func (w *FileWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = true
	w.mu.Unlock()

	_ = w.scan()

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.scan()
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// Stop 停止监控
func (w *FileWatcher) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

func (w *FileWatcher) scan() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var changedFiles []string
	paths := []string{
		filepath.Join(w.dir, "MEMORY.md"),
		filepath.Join(w.dir, "memory"),
	}

	for _, p := range paths {
		entries, err := os.ReadDir(p)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fullPath := filepath.Join(p, entry.Name())
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			if prev, ok := w.lastHashes[fullPath]; !ok || prev != hash {
				w.lastHashes[fullPath] = hash
				changedFiles = append(changedFiles, fullPath)
			}
		}
	}

	if len(changedFiles) > 0 && w.onChange != nil {
		go w.onChange(changedFiles)
	}
	return nil
}
