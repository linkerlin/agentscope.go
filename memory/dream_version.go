package memory

import (
	"context"
	"fmt"
	"time"
)

// DreamLearningHistory 记录 Dream 演化的学习历史
type DreamLearningHistory struct {
	Entries []DreamHistoryEntry `json:"entries"`
}

// DreamHistoryEntry 单条学习历史记录
type DreamHistoryEntry struct {
	Timestamp    time.Time    `json:"timestamp"`
	Action       DreamAction  `json:"action"`
	Candidate    string       `json:"candidate"`
	ExistingID   string       `json:"existing_id,omitempty"`
	Reason       string       `json:"reason"`
	ScoreChange  float64      `json:"score_change,omitempty"`
	ContentDiff  string       `json:"content_diff,omitempty"`
}

// DreamVersionManager Dream 版本管理器
type DreamVersionManager struct {
	store     VectorStore
	history   *DreamLearningHistory
	maxVersions int // 保留的最大版本数
}

// NewDreamVersionManager 创建版本管理器
func NewDreamVersionManager(store VectorStore, maxVersions int) *DreamVersionManager {
	if maxVersions <= 0 {
		maxVersions = 10
	}
	return &DreamVersionManager{
		store:       store,
		history:     &DreamLearningHistory{Entries: make([]DreamHistoryEntry, 0)},
		maxVersions: maxVersions,
	}
}

// RecordDecision 记录决策历史
func (dvm *DreamVersionManager) RecordDecision(decision *DreamDecision) {
	if dvm == nil || decision == nil {
		return
	}
	
	entry := DreamHistoryEntry{
		Timestamp:  time.Now(),
		Action:     decision.Action,
		Candidate:  decision.Candidate.Content,
		Reason:     decision.Reason,
	}
	
	if decision.Existing != nil {
		entry.ExistingID = decision.Existing.MemoryID
		if decision.Action == DreamCorroborate {
			entry.ScoreChange = decision.Candidate.Score * 0.1
		}
	}
	
	if decision.Updated != nil {
		entry.ContentDiff = diffContent(decision.Existing.Content, decision.Updated.Content)
	}
	
	dvm.history.Entries = append(dvm.history.Entries, entry)
	
	// 限制历史条目数
	if len(dvm.history.Entries) > dvm.maxVersions {
		dvm.history.Entries = dvm.history.Entries[len(dvm.history.Entries)-dvm.maxVersions:]
	}
}

// GetHistory 获取学习历史
func (dvm *DreamVersionManager) GetHistory() *DreamLearningHistory {
	if dvm == nil {
		return nil
	}
	return dvm.history
}

// GetHistoryForNode 获取指定节点的演化历史
func (dvm *DreamVersionManager) GetHistoryForNode(nodeID string) []DreamHistoryEntry {
	if dvm == nil || nodeID == "" {
		return nil
	}
	
	var result []DreamHistoryEntry
	for _, entry := range dvm.history.Entries {
		if entry.ExistingID == nodeID {
			result = append(result, entry)
		}
	}
	return result
}

// GenerateLearningReport 生成学习报告
func (dvm *DreamVersionManager) GenerateLearningReport() string {
	if dvm == nil || len(dvm.history.Entries) == 0 {
		return "No learning history yet."
	}
	
	stats := make(map[DreamAction]int)
	for _, entry := range dvm.history.Entries {
		stats[entry.Action]++
	}
	
	report := "Dream Learning Report:\n"
	report += fmt.Sprintf("  Total Decisions: %d\n", len(dvm.history.Entries))
	report += fmt.Sprintf("  CREATE: %d\n", stats[DreamCreate])
	report += fmt.Sprintf("  CORROBORATE: %d\n", stats[DreamCorroborate])
	report += fmt.Sprintf("  REFINE: %d\n", stats[DreamRefine])
	report += fmt.Sprintf("  CORRECT: %d\n", stats[DreamCorrect])
	report += fmt.Sprintf("  SKIP: %d\n", stats[DreamSkip])
	
	// 最近 5 条记录
	report += "\nRecent Decisions:\n"
	start := len(dvm.history.Entries) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(dvm.history.Entries); i++ {
		entry := dvm.history.Entries[i]
		report += fmt.Sprintf("  [%s] %s: %s\n", 
			entry.Timestamp.Format("2006-01-02 15:04"),
			entry.Action,
			entry.Reason)
	}
	
	return report
}

// diffContent 计算内容差异（简化版）
func diffContent(old, new string) string {
	if old == new {
		return "no change"
	}
	if len(new) > len(old) {
		return fmt.Sprintf("expanded: +%d chars", len(new)-len(old))
	}
	if len(new) < len(old) {
		return fmt.Sprintf("condensed: -%d chars", len(old)-len(new))
	}
	return "modified"
}

// DreamScheduler Dream 调度器
type DreamScheduler struct {
	step      *DreamStep
	vm        *DreamVersionManager
	interval  time.Duration
	lastRun   time.Time
	enabled   bool
}

// NewDreamScheduler 创建 Dream 调度器
func NewDreamScheduler(step *DreamStep, vm *DreamVersionManager, interval time.Duration) *DreamScheduler {
	return &DreamScheduler{
		step:     step,
		vm:       vm,
		interval: interval,
		enabled:  true,
	}
}

// Start 启动定时调度
func (ds *DreamScheduler) Start(ctx context.Context) {
	if !ds.enabled {
		return
	}
	
	go func() {
		ticker := time.NewTicker(ds.interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				ds.Run(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Run 手动执行一次 Dream
func (ds *DreamScheduler) Run(ctx context.Context) (*DreamResult, error) {
	if ds.step == nil {
		return nil, fmt.Errorf("dream step not configured")
	}
	
	result, err := ds.step.Execute(ctx)
	if err != nil {
		return nil, err
	}
	
	// 记录所有决策历史
	if ds.vm != nil {
		for _, node := range result.Created {
			ds.vm.RecordDecision(&DreamDecision{
				Candidate: &DreamCandidate{Content: node.Content},
				Action:    DreamCreate,
				Reason:    "scheduled dream execution",
			})
		}
	}
	
	ds.lastRun = time.Now()
	return result, nil
}

// TriggerOnThreshold 当新记忆达到阈值时触发 Dream
func (ds *DreamScheduler) TriggerOnThreshold(ctx context.Context, newMemoryCount int, threshold int) (*DreamResult, error) {
	if newMemoryCount >= threshold {
		return ds.Run(ctx)
	}
	return nil, nil
}

// IsEnabled 返回是否启用
func (ds *DreamScheduler) IsEnabled() bool {
	return ds.enabled
}

// SetEnabled 设置启用状态
func (ds *DreamScheduler) SetEnabled(enabled bool) {
	ds.enabled = enabled
}

// LastRun 返回上次运行时间
func (ds *DreamScheduler) LastRun() time.Time {
	return ds.lastRun
}
