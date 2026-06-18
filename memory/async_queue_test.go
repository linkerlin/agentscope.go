package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/memory/vector"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAsyncTaskQueue 测试异步任务队列
func TestAsyncTaskQueue(t *testing.T) {
	q := NewAsyncTaskQueue(2)
	defer q.Stop()

	// 注册测试处理器
	var executed bool
	q.RegisterHandler(TaskTypeSummarize, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		executed = true
		return &AsyncTaskResult{TaskID: task.ID, Output: "done"}, nil
	})

	// 提交任务
	taskID := q.Submit(&AsyncTask{
		Type:     TaskTypeSummarize,
		Priority: 5,
		Payload:  "test",
	})
	require.NotEmpty(t, taskID)

	// 等待任务执行
	time.Sleep(300 * time.Millisecond)

	// 验证结果
	result := q.GetResult(taskID)
	require.NotNil(t, result)
	assert.True(t, executed)
	assert.Equal(t, "done", result.Output)
	assert.True(t, result.Success)
}

// TestAsyncTaskQueuePriority 测试优先级排序
func TestAsyncTaskQueuePriority(t *testing.T) {
	q := NewAsyncTaskQueue(1)
	defer q.Stop()

	var order []int
	var mu sync.Mutex
	// Gate execution until all tasks are queued. The worker starts running as
	// soon as the queue is created, so without this gate it could dequeue the
	// first-submitted (priority 1) task before the higher-priority tasks are
	// submitted, making the test flaky under load.
	ready := make(chan struct{})
	q.RegisterHandler(TaskTypeSummarize, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		<-ready
		mu.Lock()
		order = append(order, task.Priority)
		mu.Unlock()
		return &AsyncTaskResult{TaskID: task.ID}, nil
	})

	// 按低优先级提交，高优先级应该先执行
	q.Submit(&AsyncTask{Type: TaskTypeSummarize, Priority: 1})
	q.Submit(&AsyncTask{Type: TaskTypeSummarize, Priority: 10})
	q.Submit(&AsyncTask{Type: TaskTypeSummarize, Priority: 5})
	close(ready) // all three are now queued; worker processes in priority order

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, order, 3)
	// 验证高优先级先执行
	assert.Equal(t, 10, order[0], "highest priority should execute first")
	// 其余两个顺序取决于插入和调度
	assert.Contains(t, []int{1, 5}, order[1])
	assert.Contains(t, []int{1, 5}, order[2])
}

// TestAsyncTaskQueueStats 测试队列统计
func TestAsyncTaskQueueStats(t *testing.T) {
	q := NewAsyncTaskQueue(1)
	defer q.Stop()

	q.RegisterHandler(TaskTypeSummarize, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		time.Sleep(50 * time.Millisecond)
		return &AsyncTaskResult{TaskID: task.ID}, nil
	})

	// 提交多个任务
	for i := 0; i < 5; i++ {
		q.Submit(&AsyncTask{Type: TaskTypeSummarize, Priority: i})
	}

	stats := q.Stats()
	assert.Equal(t, 1, stats["workers"])
	assert.GreaterOrEqual(t, stats["pending"], 0)
}

// TestAsyncTaskQueueRetry 测试失败重试
func TestAsyncTaskQueueRetry(t *testing.T) {
	q := NewAsyncTaskQueue(1)
	defer q.Stop()

	attempts := 0
	q.RegisterHandler(TaskTypeSummarize, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		attempts++
		if attempts < 3 {
			return nil, assert.AnError
		}
		return &AsyncTaskResult{TaskID: task.ID, Success: true}, nil
	})

	taskID := q.Submit(&AsyncTask{
		Type:       TaskTypeSummarize,
		Priority:   5,
		MaxRetries: 3,
	})

	// 等待重试完成
	time.Sleep(1 * time.Second)

	result := q.GetResult(taskID)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, attempts, 3)
}

// TestContextCheckerCompleteness 测试上下文完整性检查
func TestContextCheckerCompleteness(t *testing.T) {
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("Tool: search").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("What is the weather?").Build(),
		message.NewMsg().Role(message.RoleAssistant).Content(&message.ToolUseBlock{
			ID:   "tool-1",
			Name: "search",
		}).Build(),
	}

	report, err := CheckContextCompleteness(context.Background(), msgs, nil)
	require.NoError(t, err)
	require.NotNil(t, report)

	// 检查工具对齐
	require.NotNil(t, report.ToolAlignment)
	assert.False(t, report.ToolAlignment.IsAligned, "missing tool_result for tool_use")
	assert.Len(t, report.ToolAlignment.MismatchedCalls, 1)

	// 检查建议
	assert.NotEmpty(t, report.Recommendations)
}

// TestContextCheckerToolAlignment 测试工具对齐检查
func TestContextCheckerToolAlignment(t *testing.T) {
	// 完整工具调用
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("Tool: search\nTool: calc").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("Calculate 2+2").Build(),
		message.NewMsg().Role(message.RoleAssistant).Content(&message.ToolUseBlock{
			ID:   "tool-1",
			Name: "calc",
		}).Build(),
		message.NewMsg().Role(message.RoleUser).Content(
			message.NewToolResultBlock("tool-1", []message.ContentBlock{message.NewTextBlock("4")}, false),
		).Build(),
	}

	report, err := CheckContextCompleteness(context.Background(), msgs, nil)
	require.NoError(t, err)
	require.NotNil(t, report.ToolAlignment)
	assert.True(t, report.ToolAlignment.IsAligned)
	assert.Equal(t, 1.0, report.ToolAlignment.Score)
}

// TestContextCheckerSemanticDrift 测试语义漂移检测
func TestContextCheckerSemanticDrift(t *testing.T) {
	// 话题一致
	consistent := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("Tell me about Go programming language features").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("Go is a great language with goroutines").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("What about goroutines and channels?").Build(),
	}

	drift1 := detectSemanticDrift(consistent)
	assert.Less(t, drift1, 0.6)

	// 话题跳跃
	jumping := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("Tell me about Go programming language features").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("Go is a great language with goroutines").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("What is the capital city of France?").Build(),
	}

	drift2 := detectSemanticDrift(jumping)
	assert.GreaterOrEqual(t, drift2, drift1)
}

// TestMemoryGCMixedStrategy 测试 GC 混合策略
func TestMemoryGCMixedStrategy(t *testing.T) {
	store := NewRawVectorStore(nil)
	gc := NewMemoryCollector(store)

	// 设置混合策略
	gc.Strategy = GCMixedStrategy{
		LRUWeight:   0.4,
		LFUWeight:   0.3,
		TTLWeight:   0.2,
		ScoreWeight: 0.1,
	}
	gc.MinKeepCount = 1
	gc.BatchSize = 10
	gc.PreserveTypes = []MemoryType{}

	// 创建测试节点
	now := time.Now()
	nodes := []*vector.MemoryNode{
		{
			MemoryID:   "old-low-freq",
			MemoryType: vector.MemoryTypePersonal,
			Score:      0.1,
			Metadata: map[string]any{
				"freq":          1,
				"last_accessed": now.Add(-30 * 24 * time.Hour).Unix(),
			},
		},
		{
			MemoryID:   "recent-high-freq",
			MemoryType: vector.MemoryTypePersonal,
			Score:      0.8,
			Metadata: map[string]any{
				"freq":          10,
				"last_accessed": now.Unix(),
			},
		},
		{
			MemoryID:   "protected",
			MemoryType: vector.MemoryTypeHistory,
			Score:      0.5,
			Metadata: map[string]any{
				"freq":          5,
				"last_accessed": now.Add(-10 * 24 * time.Hour).Unix(),
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, store.Insert(ctx, nodes))

	// 执行 GC - RawVectorStore 的 Search 返回 nil，所以不会删除任何节点
	removed, err := gc.Collect(ctx)
	require.NoError(t, err)
	// 由于 RawVectorStore 返回空，我们验证函数正常执行
	assert.Empty(t, removed)
}

// TestMemoryGCAdaptiveThresholds 测试自适应阈值
func TestMemoryGCAdaptiveThresholds(t *testing.T) {
	store := NewRawVectorStore(nil)
	gc := NewMemoryCollector(store)
	gc.AdaptiveMode = true
	gc.MinKeepCount = 1

	// 创建低效用节点
	now := time.Now()
	nodes := []*vector.MemoryNode{
		{
			MemoryID: "low-utility-1",
			Score:    0.05,
			Metadata: map[string]any{
				"freq":          1,
				"utility":       0.05,
				"last_accessed": now.Unix(),
			},
		},
		{
			MemoryID: "low-utility-2",
			Score:    0.03,
			Metadata: map[string]any{
				"freq":          1,
				"utility":       0.03,
				"last_accessed": now.Unix(),
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, store.Insert(ctx, nodes))

	// 记录初始阈值
	initialThreshold := gc.UtilityThreshold

	// 执行 GC（触发自适应调整）
	_, err := gc.Collect(ctx)
	require.NoError(t, err)

	// 验证阈值被调整（低平均效用应该降低阈值）
	assert.LessOrEqual(t, gc.UtilityThreshold, initialThreshold)
}

// TestMemoryGCStats 测试 GC 统计
func TestMemoryGCStats(t *testing.T) {
	store := NewRawVectorStore(nil)
	gc := NewMemoryCollector(store)

	now := time.Now()
	nodes := []*vector.MemoryNode{
		{
			MemoryID:   "node-1",
			MemoryType: vector.MemoryTypePersonal,
			Score:      0.5,
			Metadata: map[string]any{
				"freq":          5,
				"utility":       0.5,
				"last_accessed": now.Unix(),
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, store.Insert(ctx, nodes))

	stats, err := gc.Stats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// RawVectorStore 的 Search 返回 nil，所以统计为 0
	assert.Equal(t, 0, stats.TotalNodes)
	assert.GreaterOrEqual(t, stats.AvgUtility, 0.0)
}

// TestMemoryGCCollectWithStrategy 测试指定策略的 GC
func TestMemoryGCCollectWithStrategy(t *testing.T) {
	store := NewRawVectorStore(nil)
	gc := NewMemoryCollector(store)

	now := time.Now()
	nodes := []*vector.MemoryNode{
		{
			MemoryID: "old-node",
			Score:    0.5,
			Metadata: map[string]any{
				"last_accessed": now.Add(-60 * 24 * time.Hour).Unix(),
			},
		},
		{
			MemoryID: "recent-node",
			Score:    0.5,
			Metadata: map[string]any{
				"last_accessed": now.Unix(),
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, store.Insert(ctx, nodes))

	// 使用 30 天策略
	strategy := GCStrategy{
		MaxAge:   30 * 24 * time.Hour,
		MinScore: 0.0,
	}

	removed, err := gc.CollectWithStrategy(ctx, strategy)
	require.NoError(t, err)
	// RawVectorStore 的 Search 返回 nil，所以逻辑上不会删除任何节点
	// 但函数本身应该正常执行
	assert.Empty(t, removed)
}

// TestSummarizeTaskHandler 测试摘要任务处理器
func TestSummarizeTaskHandler(t *testing.T) {
	// 创建 mock summarizer
	summarizer := &Summarizer{}

	handler := SummarizeTaskHandler(summarizer)
	require.NotNil(t, handler)

	// 由于 summarizer 没有 model，会返回错误
	// 这里主要测试 handler 的创建和基本流程
	task := &AsyncTask{
		ID:      "test-task",
		Type:    TaskTypeSummarize,
		Payload: map[string]string{"memory_id": "m1", "content": "test content"},
	}

	// 没有 model 的情况下会报错，这是预期行为
	_, err := handler(context.Background(), task)
	assert.Error(t, err)
}

// TestAsyncTaskQueueIntegration 集成测试：完整工作流
func TestAsyncTaskQueueIntegration(t *testing.T) {
	q := NewAsyncTaskQueue(1)
	defer q.Stop()

	var sumOutput, gcOutput any
	var mu sync.Mutex

	// 注册多个处理器
	q.RegisterHandler(TaskTypeSummarize, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		mu.Lock()
		sumOutput = "summarized"
		mu.Unlock()
		return &AsyncTaskResult{TaskID: task.ID, Output: "summarized"}, nil
	})
	q.RegisterHandler(TaskTypeGC, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		mu.Lock()
		gcOutput = map[string]any{"removed": 5}
		mu.Unlock()
		return &AsyncTaskResult{TaskID: task.ID, Output: map[string]any{"removed": 5}}, nil
	})

	// 提交多种任务（使用不同优先级确保顺序可预测）
	sumID := q.SubmitSummarize("mem-1", "content", 10)
	gcID := q.SubmitGC(GCStrategy{MaxAge: 24 * time.Hour}, 5)

	// 等待执行
	time.Sleep(1 * time.Second)

	// 验证结果 - 两个任务都应该完成
	sumResult := q.GetResult(sumID)
	gcResult := q.GetResult(gcID)

	// 两个任务都应该完成
	require.NotNil(t, sumResult, "Summarize 任务应该完成")
	require.NotNil(t, gcResult, "GC 任务应该完成")

	// 验证各自输出
	mu.Lock()
	assert.Equal(t, "summarized", sumOutput)
	gcMap, ok := gcOutput.(map[string]any)
	mu.Unlock()
	require.True(t, ok)
	assert.Equal(t, 5, gcMap["removed"])
}
