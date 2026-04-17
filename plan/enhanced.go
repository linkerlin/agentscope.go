package plan

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkerlin/agentscope.go/state"
)

// PlanState 计划整体状态
type PlanState string

const (
	PlanStateActive    PlanState = "active"
	PlanStateDone      PlanState = "done"
	PlanStateAbandoned PlanState = "abandoned"
)

// SubtaskState 子任务状态
type SubtaskState string

const (
	SubtaskTodo       SubtaskState = "todo"
	SubtaskInProgress SubtaskState = "in_progress"
	SubtaskDone       SubtaskState = "done"
	SubtaskAbandoned  SubtaskState = "abandoned"
)

// Subtask 子任务
type Subtask struct {
	ID              string
	Name            string
	Description     string
	ExpectedOutcome string
	State           SubtaskState
	Outcome         string
	Dependencies    []string
}

// RichPlan 增强计划结构
type RichPlan struct {
	ID              string
	Name            string
	Description     string
	ExpectedOutcome string
	State           PlanState
	Subtasks        []Subtask
	CreatedAt       time.Time
	UpdatedAt       time.Time
	mu              sync.RWMutex
}

// StateType 实现 state.State 接口，用于持久化
func (p *RichPlan) StateType() string { return "rich_plan" }

// EnhancedPlanNotebook 带历史与提示的计划本
type EnhancedPlanNotebook struct {
	mu              sync.RWMutex
	current         *RichPlan
	history         []*RichPlan
	historyByID     map[string]*RichPlan
	defaultHintFunc func(*RichPlan) string
	store           state.Store
}

// NewEnhancedPlanNotebook 创建增强计划本
func NewEnhancedPlanNotebook() *EnhancedPlanNotebook {
	return &EnhancedPlanNotebook{
		historyByID: make(map[string]*RichPlan),
	}
}

// NewEnhancedPlanNotebookWithStore 创建带持久化存储的增强计划本，并尝试从存储恢复
func NewEnhancedPlanNotebookWithStore(store state.Store) *EnhancedPlanNotebook {
	nb := NewEnhancedPlanNotebook()
	nb.store = store
	_ = nb.LoadFromStore()
	return nb
}

// LoadFromStore 从 state.Store 恢复所有历史计划
func (nb *EnhancedPlanNotebook) LoadFromStore() error {
	if nb.store == nil {
		return nil
	}
	nb.mu.Lock()
	defer nb.mu.Unlock()

	keys := nb.store.ListKeys()
	for _, key := range keys {
		if !strings.HasPrefix(key, "plan_") {
			continue
		}
		var p RichPlan
		if err := nb.store.Get(key, &p); err != nil {
			continue
		}
		if p.ID == "" {
			continue
		}
		nb.historyByID[p.ID] = &p
		nb.history = append(nb.history, &p)
		if nb.current == nil || p.UpdatedAt.After(nb.current.UpdatedAt) {
			nb.current = &p
		}
	}
	return nil
}

func (nb *EnhancedPlanNotebook) persistCurrent() {
	if nb.store == nil || nb.current == nil {
		return
	}
	_ = nb.store.Save("plan_"+nb.current.ID, nb.current)
}

// SetHintFunc 自定义根据当前计划生成注入提示的函数
func (nb *EnhancedPlanNotebook) SetHintFunc(fn func(*RichPlan) string) {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	nb.defaultHintFunc = fn
}

// CreatePlan 创建当前计划并可选子任务
func (nb *EnhancedPlanNotebook) CreatePlan(name, description, expected string, subtasks []Subtask) (*RichPlan, error) {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	p := &RichPlan{
		ID:              uuid.New().String(),
		Name:            name,
		Description:     description,
		ExpectedOutcome: expected,
		State:           PlanStateActive,
		Subtasks:        append([]Subtask(nil), subtasks...),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	for i := range p.Subtasks {
		if p.Subtasks[i].ID == "" {
			p.Subtasks[i].ID = uuid.New().String()
		}
		if p.Subtasks[i].State == "" {
			p.Subtasks[i].State = SubtaskTodo
		}
	}
	nb.current = p
	nb.historyByID[p.ID] = p
	nb.history = append(nb.history, p)
	nb.persistCurrent()
	return p, nil
}

// CurrentPlan 返回当前活跃计划
func (nb *EnhancedPlanNotebook) CurrentPlan() *RichPlan {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	return nb.current
}

// UpdateSubtaskState 更新子任务状态
func (nb *EnhancedPlanNotebook) UpdateSubtaskState(idx int, st SubtaskState) error {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	if nb.current == nil {
		return fmt.Errorf("plan: no current plan")
	}
	nb.current.mu.Lock()
	defer nb.current.mu.Unlock()
	if idx < 0 || idx >= len(nb.current.Subtasks) {
		return fmt.Errorf("plan: invalid subtask index")
	}
	nb.current.Subtasks[idx].State = st
	nb.current.UpdatedAt = time.Now()
	nb.persistCurrent()
	return nil
}

// FinishSubtask 完成子任务并记录结果
func (nb *EnhancedPlanNotebook) FinishSubtask(idx int, outcome string) error {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	if nb.current == nil {
		return fmt.Errorf("plan: no current plan")
	}
	nb.current.mu.Lock()
	defer nb.current.mu.Unlock()
	if idx < 0 || idx >= len(nb.current.Subtasks) {
		return fmt.Errorf("plan: invalid subtask index")
	}
	nb.current.Subtasks[idx].State = SubtaskDone
	nb.current.Subtasks[idx].Outcome = outcome
	nb.current.UpdatedAt = time.Now()
	nb.persistCurrent()
	return nil
}

// FinishPlan 结束当前计划
func (nb *EnhancedPlanNotebook) FinishPlan(state PlanState, summary string) error {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	if nb.current == nil {
		return fmt.Errorf("plan: no current plan")
	}
	nb.current.mu.Lock()
	nb.current.State = state
	nb.current.UpdatedAt = time.Now()
	nb.current.mu.Unlock()
	_ = summary
	nb.persistCurrent()
	return nil
}

// ViewHistoricalPlans 返回历史计划 ID 列表快照
func (nb *EnhancedPlanNotebook) ViewHistoricalPlans() []string {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	out := make([]string, 0, len(nb.history))
	for _, p := range nb.history {
		if p != nil {
			out = append(out, p.ID)
		}
	}
	return out
}

// RecoverHistoricalPlan 将指定 ID 的计划设为当前（浅拷贝引用）
func (nb *EnhancedPlanNotebook) RecoverHistoricalPlan(planID string) error {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	p, ok := nb.historyByID[planID]
	if !ok {
		return fmt.Errorf("plan: not found: %s", planID)
	}
	nb.current = p
	return nil
}

// GenerateHint 生成可注入模型的提示文本
func (nb *EnhancedPlanNotebook) GenerateHint() string {
	nb.mu.RLock()
	fn := nb.defaultHintFunc
	cur := nb.current
	nb.mu.RUnlock()
	if fn != nil {
		return fn(cur)
	}
	if cur == nil {
		return ""
	}
	cur.mu.RLock()
	defer cur.mu.RUnlock()
	var b []byte
	b = append(b, "<system-hint>\n"...)
	b = append(b, "计划: "...)
	b = append(b, cur.Name...)
	b = append(b, "\n子任务:\n"...)
	for i, st := range cur.Subtasks {
		b = append(b, fmt.Sprintf("  [%d] %s - %s\n", i, st.Name, st.State)...)
	}
	b = append(b, "</system-hint>"...)
	return string(b)
}
