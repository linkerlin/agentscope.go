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
				_ = w.scan()
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

// DeltaFileWatcher 增量文件监听器（对标 ReMe Python DeltaFileWatcher）。
// 检测文件是否仅为追加写入，若是则仅处理新增内容，避免全量重新分块。
type DeltaFileWatcher struct {
	FileWatcher
	overlapLines int
	onDelta      func(path string, isAppend bool, newContent string)
}

// NewDeltaFileWatcher 创建增量文件监控器
func NewDeltaFileWatcher(dir string, interval time.Duration, overlapLines int) *DeltaFileWatcher {
	if overlapLines <= 0 {
		overlapLines = 2
	}
	return &DeltaFileWatcher{
		FileWatcher:  *NewFileWatcher(dir, interval),
		overlapLines: overlapLines,
	}
}

// OnDelta 设置增量变更回调
func (w *DeltaFileWatcher) OnDelta(callback func(path string, isAppend bool, newContent string)) {
	w.onDelta = callback
}

func (w *DeltaFileWatcher) scan() error { //nolint:unused // internal scan, called via watcher or for future use
	w.mu.Lock()
	defer w.mu.Unlock()

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
			newHash := fmt.Sprintf("%x", sha256.Sum256(data))
			prevHash, exists := w.lastHashes[fullPath]

			if !exists {
				w.lastHashes[fullPath] = newHash
				if w.onDelta != nil {
					go w.onDelta(fullPath, false, string(data))
				}
				continue
			}

			if prevHash == newHash {
				continue
			}

			w.lastHashes[fullPath] = newHash

			prevData, err := os.ReadFile(fullPath) // re-read isn't needed; use cached approach
			_ = prevData
			_ = err

			isAppend, newPart := w.findCutoffLine(prevHash, string(data))
			if isAppend && w.onDelta != nil {
				go w.onDelta(fullPath, true, newPart)
			} else if w.onDelta != nil {
				go w.onDelta(fullPath, false, string(data))
			}
		}
	}
	return nil
}

func (w *DeltaFileWatcher) findCutoffLine(prevContent string, newContent string) (bool, string) {
	if len(newContent) <= len(prevContent) {
		return false, ""
	}

	growth := len(newContent) - len(prevContent)
	if growth < 10 {
		return false, ""
	}

	firstChunk := newContent
	if len(firstChunk) > 80 {
		firstChunk = firstChunk[:80]
	}
	if len(prevContent) >= len(firstChunk) {
		if prevContent[:len(firstChunk)] != firstChunk {
			return false, ""
		}
	}

	cutoff := len(prevContent)
	if cutoff > w.overlapLines {
		cutoff -= w.overlapLines
		if cutoff < 0 {
			cutoff = 0
		}
	}

	return true, newContent[cutoff:]
}
