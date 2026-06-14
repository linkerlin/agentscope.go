package memory

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AsyncTaskType 异步任务类型
type AsyncTaskType string

const (
	TaskTypeSummarize AsyncTaskType = "summarize"
	TaskTypeDream     AsyncTaskType = "dream"
	TaskTypeGC        AsyncTaskType = "gc"
	TaskTypeIndex     AsyncTaskType = "index"
	TaskTypeEmbed     AsyncTaskType = "embed"
)

// AsyncTask 异步任务
type AsyncTask struct {
	ID         string        `json:"id"`
	Type       AsyncTaskType `json:"type"`
	Priority   int           `json:"priority"` // 0-10, 越高越优先
	Payload    any           `json:"payload"`
	CreatedAt  time.Time     `json:"created_at"`
	Deadline   *time.Time    `json:"deadline,omitempty"`
	Retries    int           `json:"retries"`
	MaxRetries int           `json:"max_retries"`
}

// AsyncTaskResult 异步任务结果
type AsyncTaskResult struct {
	TaskID    string    `json:"task_id"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Output    any       `json:"output,omitempty"`
	Completed time.Time `json:"completed_at"`
}

// AsyncTaskHandler 任务处理器函数
type AsyncTaskHandler func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error)

// AsyncTaskQueue 异步任务队列
type AsyncTaskQueue struct {
	mu       sync.RWMutex
	tasks    []*AsyncTask
	handlers map[AsyncTaskType]AsyncTaskHandler
	results  map[string]*AsyncTaskResult
	running  map[string]bool
	workers  int
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewAsyncTaskQueue 创建异步任务队列
func NewAsyncTaskQueue(workers int) *AsyncTaskQueue {
	if workers <= 0 {
		workers = 4
	}
	ctx, cancel := context.WithCancel(context.Background())
	q := &AsyncTaskQueue{
		handlers: make(map[AsyncTaskType]AsyncTaskHandler),
		results:  make(map[string]*AsyncTaskResult),
		running:  make(map[string]bool),
		workers:  workers,
		ctx:      ctx,
		cancel:   cancel,
	}

	// 启动工作器
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	return q
}

// RegisterHandler 注册任务处理器
func (q *AsyncTaskQueue) RegisterHandler(taskType AsyncTaskType, handler AsyncTaskHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[taskType] = handler
}

// Submit 提交任务
func (q *AsyncTaskQueue) Submit(task *AsyncTask) string {
	if task == nil {
		return ""
	}

	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.MaxRetries <= 0 {
		task.MaxRetries = 3
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// 按优先级插入（高优先级在前）
	inserted := false
	for i, t := range q.tasks {
		if task.Priority > t.Priority {
			q.tasks = append(q.tasks[:i], append([]*AsyncTask{task}, q.tasks[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		q.tasks = append(q.tasks, task)
	}

	return task.ID
}

// SubmitSummarize 提交摘要任务
func (q *AsyncTaskQueue) SubmitSummarize(memoryID string, content string, priority int) string {
	return q.Submit(&AsyncTask{
		Type:       TaskTypeSummarize,
		Priority:   priority,
		Payload:    map[string]string{"memory_id": memoryID, "content": content},
		MaxRetries: 2,
	})
}

// SubmitDream 提交 Dream 演化任务
func (q *AsyncTaskQueue) SubmitDream(cfg DreamConfig, priority int) string {
	return q.Submit(&AsyncTask{
		Type:       TaskTypeDream,
		Priority:   priority,
		Payload:    cfg,
		MaxRetries: 1,
	})
}

// SubmitGC 提交垃圾回收任务
func (q *AsyncTaskQueue) SubmitGC(strategy GCStrategy, priority int) string {
	return q.Submit(&AsyncTask{
		Type:       TaskTypeGC,
		Priority:   priority,
		Payload:    strategy,
		MaxRetries: 3,
	})
}

// GetResult 获取任务结果
func (q *AsyncTaskQueue) GetResult(taskID string) *AsyncTaskResult {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.results[taskID]
}

// IsRunning 检查任务是否正在执行
func (q *AsyncTaskQueue) IsRunning(taskID string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.running[taskID]
}

// Stop 停止队列
func (q *AsyncTaskQueue) Stop() {
	q.cancel()
	q.wg.Wait()
}

// worker 工作器循环
func (q *AsyncTaskQueue) worker(id int) {
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		default:
		}

		q.mu.Lock()
		if len(q.tasks) == 0 {
			q.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 取出第一个任务
		task := q.tasks[0]
		q.tasks = q.tasks[1:]
		q.running[task.ID] = true
		q.mu.Unlock()

		// 执行任务
		result, err := q.executeTask(task)

		// 创建新的 result 对象存储，避免共享指针问题
		storedResult := &AsyncTaskResult{
			TaskID:    result.TaskID,
			Success:   result.Success,
			Error:     result.Error,
			Output:    result.Output,
			Completed: result.Completed,
		}

		q.mu.Lock()
		delete(q.running, task.ID)
		if storedResult != nil {
			q.results[task.ID] = storedResult
		}

		// 如果失败且未超过重试次数，重新入队
		if err != nil && task.Retries < task.MaxRetries {
			task.Retries++
			task.Priority-- // 降低优先级
			q.tasks = append(q.tasks, task)
		}
		q.mu.Unlock()
	}
}

// executeTask 执行单个任务
func (q *AsyncTaskQueue) executeTask(task *AsyncTask) (*AsyncTaskResult, error) {
	q.mu.RLock()
	handler, ok := q.handlers[task.Type]
	q.mu.RUnlock()

	if !ok {
		return &AsyncTaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Sprintf("no handler for task type: %s", task.Type),
		}, fmt.Errorf("no handler for task type: %s", task.Type)
	}

	// 检查截止时间
	if task.Deadline != nil && time.Now().After(*task.Deadline) {
		return &AsyncTaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   "task deadline exceeded",
		}, fmt.Errorf("task deadline exceeded")
	}

	ctx, cancel := context.WithTimeout(q.ctx, 5*time.Minute)
	defer cancel()

	result, err := handler(ctx, task)
	if result == nil {
		result = &AsyncTaskResult{TaskID: task.ID}
	}
	result.Completed = time.Now()

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	return result, err
}

// Stats 返回队列统计
func (q *AsyncTaskQueue) Stats() map[string]any {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return map[string]any{
		"pending":   len(q.tasks),
		"running":   len(q.running),
		"completed": len(q.results),
		"workers":   q.workers,
	}
}

// SummarizeTaskHandler 创建摘要任务处理器
func SummarizeTaskHandler(summarizer *Summarizer) AsyncTaskHandler {
	return func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		payload, ok := task.Payload.(map[string]string)
		if !ok {
			return nil, fmt.Errorf("invalid payload type")
		}

		content := payload["content"]
		memoryID := payload["memory_id"]

		result, err := summarizer.Summarize(ctx, content)
		if err != nil {
			return nil, err
		}

		return &AsyncTaskResult{
			TaskID: task.ID,
			Output: map[string]string{
				"memory_id": memoryID,
				"summary":   result,
			},
		}, nil
	}
}

// DreamTaskHandler 创建 Dream 任务处理器
func DreamTaskHandler(dreamStep *DreamStep, vm *DreamVersionManager) AsyncTaskHandler {
	return func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		result, err := dreamStep.Execute(ctx)
		if err != nil {
			return nil, err
		}

		// 记录历史
		if vm != nil {
			for _, node := range result.Created {
				vm.RecordDecision(&DreamDecision{
					Candidate: &DreamCandidate{Content: node.Content},
					Action:    DreamCreate,
					Reason:    "async dream execution",
				})
			}
		}

		return &AsyncTaskResult{
			TaskID: task.ID,
			Output: map[string]any{
				"created":      len(result.Created),
				"corroborated": len(result.Corroborated),
				"refined":      len(result.Refined),
				"corrected":    len(result.Corrected),
				"skipped":      len(result.Skipped),
			},
		}, nil
	}
}

// GCTaskHandler 创建垃圾回收任务处理器
func GCTaskHandler(gc *MemoryCollector) AsyncTaskHandler {
	return func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		// 支持通过 Payload 指定策略，否则使用默认策略
		if strategy, ok := task.Payload.(GCStrategy); ok {
			result, err := gc.CollectWithStrategy(ctx, strategy)
			if err != nil {
				return nil, err
			}
			return &AsyncTaskResult{
				TaskID: task.ID,
				Output: map[string]any{
					"removed": result,
				},
			}, nil
		}

		result, err := gc.Collect(ctx)
		if err != nil {
			return nil, err
		}

		return &AsyncTaskResult{
			TaskID: task.ID,
			Output: map[string]any{
				"removed": result,
			},
		}, nil
	}
}

// GCStrategy 垃圾回收策略
type GCStrategy struct {
	MaxAge        time.Duration `json:"max_age"`
	MinScore      float64       `json:"min_score"`
	MaxCount      int           `json:"max_count"`
	PreserveTypes []MemoryType  `json:"preserve_types"`
}
