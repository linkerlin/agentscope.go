package memory

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// asyncSummaryTask 异步摘要任务
type asyncSummaryTask struct {
	done chan struct{}
	err  error
}

// ReMeFileMemory 文件型 ReMe 记忆（ReMeLight 对齐）
type ReMeFileMemory struct {
	*ReMeInMemoryMemory

	workingPath    string
	memoryPath     string
	toolResultPath string
	dialogPath     string
	sessionsPath   string
	agentscopePath string

	tokenCounter TokenCounter
	compactor    *Compactor
	summarizer   *Summarizer
	toolCompact  *ToolResultCompactor
	config       ReMeFileConfig
	fts          *FTSIndex

	summaryMu    sync.Mutex
	summaryTasks []*asyncSummaryTask
}

// NewReMeFileMemory 创建工作目录结构并返回实例
func NewReMeFileMemory(cfg ReMeFileConfig, counter TokenCounter) (*ReMeFileMemory, error) {
	if cfg.WorkingDir == "" {
		cfg = DefaultReMeFileConfig()
	}
	if counter == nil {
		counter = NewSimpleTokenCounter()
	}
	base := cfg.WorkingDir
	dirs := []string{
		base,
		filepath.Join(base, "memory"),
		filepath.Join(base, "dialog"),
		filepath.Join(base, "tool_result"),
		filepath.Join(base, "sessions"),
		filepath.Join(base, ".agentscope"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	ftsIndex, _ := NewFTSIndex(filepath.Join(base, ".agentscope", "reme.db"))

	m := &ReMeFileMemory{
		ReMeInMemoryMemory: NewReMeInMemoryMemory(filepath.Join(base, "dialog")),
		workingPath:        base,
		memoryPath:         filepath.Join(base, "memory"),
		dialogPath:         filepath.Join(base, "dialog"),
		toolResultPath:     filepath.Join(base, "tool_result"),
		sessionsPath:       filepath.Join(base, "sessions"),
		agentscopePath:     filepath.Join(base, ".agentscope"),
		tokenCounter:       counter,
		config:             cfg,
		fts:                ftsIndex,
	}
	m.toolCompact = NewToolResultCompactor(m.toolResultPath, cfg.RecentMaxBytes, cfg.OldMaxBytes, cfg.ToolResultRetentionDays)
	return m, nil
}

// InitCompactorWithModel 注入用于压缩摘要的 ChatModel
func (m *ReMeFileMemory) InitCompactorWithModel(cm model.ChatModel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cm == nil {
		m.compactor = nil
		return
	}
	m.compactor = NewCompactor(cm)
}

// InitSummarizerWithModel 注入用于持久化摘要的 ChatModel
func (m *ReMeFileMemory) InitSummarizerWithModel(cm model.ChatModel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cm == nil {
		m.summarizer = nil
		return
	}
	m.summarizer = &Summarizer{Model: cm, WorkingDir: m.workingPath}
}

// CheckContext 对当前内存中的消息做上下文切分
func (m *ReMeFileMemory) CheckContext(ctx context.Context, threshold, reserve int) (*ContextCheckResult, error) {
	return CheckContext(ctx, m.Msgs(), threshold, reserve, m.tokenCounter)
}

// CompactMemory 压缩给定消息并更新内部摘要文本
func (m *ReMeFileMemory) CompactMemory(ctx context.Context, messages []*message.Msg, opts CompactOptions) (string, error) {
	if m.compactor == nil {
		return "", ErrCompactorNoModel
	}
	sum, err := m.compactor.Compact(ctx, messages, opts)
	if err != nil {
		return "", err
	}
	m.SetCompSum(sum.Raw)
	return sum.Raw, nil
}

// EstimateTokens 估算 token
func (m *ReMeFileMemory) EstimateTokens(messages []*message.Msg) (*TokenStats, error) {
	n, err := m.tokenCounter.CountMessages(messages)
	if err != nil {
		return nil, err
	}
	max := m.config.MaxInputLength
	ratio := 0.0
	if max > 0 {
		ratio = float64(n) / float64(max)
		if ratio > 1 {
			ratio = 1
		}
	}
	return &TokenStats{
		TotalMessages:     len(messages),
		EstimatedTokens: n,
		MaxInputLength:    max,
		ContextUsageRatio: ratio,
	}, nil
}

// SaveTo 持久化会话快照（摘要与标记；消息列表需依赖 dialog 追加）
func (m *ReMeFileMemory) SaveTo(sessionID string) error {
	if sessionID == "" {
		return errors.New("memory: empty session id")
	}
	snap := m.ReMeInMemoryMemory.Snapshot()
	fsnap := &sessionSnapshotV1{
		Version:           1,
		CompressedSummary: snap.CompressedSummary,
		LongTermMemory:    snap.LongTermMemory,
		Marks:             snap.Marks,
	}
	data, err := json.MarshalIndent(fsnap, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.sessionsPath, sessionID+".json")
	return os.WriteFile(path, data, 0o644)
}

// LoadFrom 加载快照
func (m *ReMeFileMemory) LoadFrom(sessionID string) error {
	if sessionID == "" {
		return errors.New("memory: empty session id")
	}
	path := filepath.Join(m.sessionsPath, sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fsnap sessionSnapshotV1
	if err := json.Unmarshal(data, &fsnap); err != nil {
		return err
	}
	m.ReMeInMemoryMemory.Restore(&InMemorySnapshot{
		CompressedSummary: fsnap.CompressedSummary,
		LongTermMemory:    fsnap.LongTermMemory,
		Marks:             fsnap.Marks,
	})
	return nil
}

type sessionSnapshotV1 struct {
	Version           int                 `json:"version"`
	CompressedSummary string              `json:"compressed_summary"`
	LongTermMemory    string              `json:"long_term_memory"`
	Marks             map[string][]string `json:"marks"`
}

// PreReasoningPrepare 在模型调用前裁剪/压缩上下文
func (m *ReMeFileMemory) PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *CompactSummary, error) {
	if len(history) == 0 {
		return history, nil, nil
	}
	h := history
	if m.toolCompact != nil {
		var err error
		h, err = m.toolCompact.Compact(h, 2)
		if err != nil {
			return nil, nil, err
		}
	}
	threshold := int(float64(m.config.MaxInputLength) * m.config.CompactRatio)
	if threshold <= 0 {
		threshold = m.config.MaxInputLength
	}
	reserve := m.config.MemoryCompactReserve
	if reserve <= 0 {
		reserve = 8000
	}
	cc, err := CheckContext(ctx, h, threshold, reserve, m.tokenCounter)
	if err != nil {
		return nil, nil, err
	}
	if len(cc.MessagesToCompact) == 0 {
		return h, nil, nil
	}
	if m.compactor == nil {
		return h, nil, nil
	}
	sum, err := m.compactor.Compact(ctx, cc.MessagesToCompact, CompactOptions{
		Language:        m.config.Language,
		PreviousSummary: m.GetCompSum(),
	})
	if err != nil {
		return nil, nil, err
	}
	m.SetCompSum(sum.Raw)

	// 触发异步摘要（非阻塞）
	if m.summarizer != nil {
		m.AddAsyncSummaryTask(ctx, cc.MessagesToCompact)
	}

	var out []*message.Msg
	if sum.Raw != "" {
		out = append(out, message.NewMsg().
			Role(message.RoleUser).
			TextContent("# Summary of previous conversation\n\n"+sum.Raw).
			Build())
	}
	out = append(out, cc.MessagesToKeep...)
	return out, sum, nil
}

// GetMemoryForPrompt 返回带长期记忆与摘要前缀的消息视图（供 buildHistory 使用）
func (m *ReMeFileMemory) GetMemoryForPrompt(prepend bool) ([]*message.Msg, error) {
	filtered, _ := m.GetAll()
	if !prepend {
		return append([]*message.Msg(nil), filtered...), nil
	}
	var parts []string
	if lt := m.GetLongTermMemory(); lt != "" {
		parts = append(parts, "# Memories\n\n"+lt)
	}
	if cs := m.GetCompSum(); cs != "" {
		parts = append(parts, "# Summary of previous conversation\n\n"+cs)
	}
	if len(parts) == 0 {
		return append([]*message.Msg(nil), filtered...), nil
	}
	sumMsg := message.NewMsg().
		Role(message.RoleUser).
		Name("user").
		TextContent(strings.Join(parts, "\n\n")).
		Build()
	return append([]*message.Msg{sumMsg}, filtered...), nil
}

// AddAsyncSummaryTask 启动后台摘要任务（将待压缩消息写入 memory/YYYY-MM-DD.md）
func (m *ReMeFileMemory) AddAsyncSummaryTask(ctx context.Context, msgs []*message.Msg) {
	if m.summarizer == nil || m.summarizer.Model == nil {
		return
	}
	task := &asyncSummaryTask{done: make(chan struct{})}
	go func() {
		task.err = m.summarizer.SummarizeToDailyFile(ctx, msgs)
		close(task.done)
	}()
	m.summaryMu.Lock()
	m.summaryTasks = append(m.summaryTasks, task)
	m.summaryMu.Unlock()
}

// FTSIndex 返回全文索引实例（可能为 nil）
func (m *ReMeFileMemory) FTSIndex() *FTSIndex {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fts
}

// AwaitSummaryTasks 等待所有后台摘要任务完成并返回第一个错误
func (m *ReMeFileMemory) AwaitSummaryTasks() error {
	m.summaryMu.Lock()
	tasks := m.summaryTasks
	m.summaryTasks = nil
	m.summaryMu.Unlock()

	var firstErr error
	for _, t := range tasks {
		<-t.done
		if t.err != nil && firstErr == nil {
			firstErr = t.err
		}
	}
	return firstErr
}

// Close 关闭资源（如 FTS 数据库连接）
func (m *ReMeFileMemory) Close() error {
	if m.fts != nil {
		return m.fts.Close()
	}
	return nil
}

var _ ReMeMemory = (*ReMeFileMemory)(nil)
