package memory

import (
	"context"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReactStepCreation 测试 ReAct 步骤创建
func TestReactStepCreation(t *testing.T) {
	step := NewReactStep(0, StepReasoning, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("What is the weather?").Build(),
	})

	require.NotNil(t, step)
	assert.Equal(t, 0, step.Iteration)
	assert.Equal(t, StepReasoning, step.Type)
	assert.NotEmpty(t, step.ID)
	assert.False(t, step.Timestamp.IsZero())
	assert.Len(t, step.Messages, 1)
}

// TestReactStepWithMemoryNodes 测试附加记忆节点
func TestReactStepWithMemoryNodes(t *testing.T) {
	step := NewReactStep(1, StepReasoning, nil)
	
	nodes := []*MemoryNode{
		{MemoryID: "mem-1", Content: "Paris is sunny", Score: 0.9},
		{MemoryID: "mem-2", Content: "London is rainy", Score: 0.8},
	}
	
	step.WithMemoryNodes(nodes)
	require.Len(t, step.MemoryNodes, 2)
	assert.Equal(t, "mem-1", step.MemoryNodes[0].MemoryID)
}

// TestReactStepWithToolCalls 测试附加工具调用
func TestReactStepWithToolCalls(t *testing.T) {
	step := NewReactStep(1, StepActing, nil)
	
	calls := []*message.ToolUseBlock{
		{ID: "tool-1", Name: "search", Input: map[string]any{"query": "weather"}},
	}
	
	step.WithToolCalls(calls)
	require.Len(t, step.ToolCalls, 1)
	assert.Equal(t, "search", step.ToolCalls[0].Name)
}

// TestInMemoryStepStore 测试内存步级存储
func TestInMemoryStepStore(t *testing.T) {
	store := NewInMemoryStepStore()
	ctx := context.Background()
	
	// 保存多个步骤
	steps := []*ReactStep{
		NewReactStep(0, StepReasoning, nil),
		NewReactStep(0, StepActing, nil),
		NewReactStep(0, StepObservation, nil),
		NewReactStep(1, StepReasoning, nil),
		NewReactStep(1, StepFinal, nil),
	}
	
	for _, step := range steps {
		require.NoError(t, store.Save(ctx, step))
	}
	
	// 测试 GetAll
	all, err := store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 5)
	
	// 测试 GetByIteration
	iter0, err := store.GetByIteration(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, iter0, 3)
	
	iter1, err := store.GetByIteration(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, iter1, 2)
	
	// 测试 GetByType
	reasoningSteps, err := store.GetByType(ctx, StepReasoning)
	require.NoError(t, err)
	assert.Len(t, reasoningSteps, 2)
	
	finalSteps, err := store.GetByType(ctx, StepFinal)
	require.NoError(t, err)
	assert.Len(t, finalSteps, 1)
	
	// 测试 Stats
	stats := store.Stats()
	assert.Equal(t, 5, stats["total"])
	assert.Equal(t, 2, stats["reasoning"])
	assert.Equal(t, 1, stats["acting"])
	assert.Equal(t, 1, stats["observation"])
	assert.Equal(t, 1, stats["final"])
	
	// 测试 Clear
	require.NoError(t, store.Clear(ctx))
	all, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

// TestReactStepRecorder 测试步级记录器
func TestReactStepRecorder(t *testing.T) {
	recorder := NewReactStepRecorder(nil)
	ctx := context.Background()
	
	// 记录 reasoning 步骤
	reasoningMsg := message.NewMsg().Role(message.RoleUser).TextContent("What is the weather?").Build()
	memNodes := []*MemoryNode{
		{MemoryID: "mem-1", Content: "Paris is sunny"},
	}
	step1, err := recorder.RecordReasoning(ctx, 0, []*message.Msg{reasoningMsg}, memNodes)
	require.NoError(t, err)
	assert.Equal(t, StepReasoning, step1.Type)
	assert.Len(t, step1.MemoryNodes, 1)
	
	// 记录 acting 步骤
	toolCalls := []*message.ToolUseBlock{
		{ID: "tool-1", Name: "search"},
	}
	step2, err := recorder.RecordActing(ctx, 0, toolCalls)
	require.NoError(t, err)
	assert.Equal(t, StepActing, step2.Type)
	assert.Len(t, step2.ToolCalls, 1)
	
	// 记录 observation 步骤
	obsMsg := message.NewMsg().Role(message.RoleTool).TextContent("The weather is sunny").Build()
	step3, err := recorder.RecordObservation(ctx, 0, []*message.Msg{obsMsg})
	require.NoError(t, err)
	assert.Equal(t, StepObservation, step3.Type)
	
	// 记录 final 步骤
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("The weather is sunny today").Build()
	step4, err := recorder.RecordFinal(ctx, 1, finalMsg)
	require.NoError(t, err)
	assert.Equal(t, StepFinal, step4.Type)
	
	// 验证所有步骤
	allSteps, err := recorder.GetAllSteps(ctx)
	require.NoError(t, err)
	assert.Len(t, allSteps, 4)
}

// TestBuildSequence 测试构建完整序列
func TestBuildSequence(t *testing.T) {
	store := NewInMemoryStepStore()
	ctx := context.Background()
	
	// 模拟完整 ReAct 循环
	steps := []*ReactStep{
		NewReactStep(0, StepReasoning, []*message.Msg{
			message.NewMsg().Role(message.RoleUser).TextContent("What is 2+2?").Build(),
		}),
		NewReactStep(0, StepActing, nil).WithToolCalls([]*message.ToolUseBlock{
			{ID: "tool-1", Name: "calc"},
		}),
		NewReactStep(0, StepObservation, []*message.Msg{
			message.NewMsg().Role(message.RoleTool).TextContent("4").Build(),
		}),
		NewReactStep(1, StepFinal, []*message.Msg{
			message.NewMsg().Role(message.RoleAssistant).TextContent("2+2=4").Build(),
		}),
	}
	
	for _, step := range steps {
		require.NoError(t, store.Save(ctx, step))
	}
	
	seq, err := BuildSequence(ctx, store, "session-1", "test-agent")
	require.NoError(t, err)
	require.NotNil(t, seq)
	
	assert.Equal(t, "session-1", seq.SessionID)
	assert.Equal(t, "test-agent", seq.AgentName)
	assert.Equal(t, 4, len(seq.Steps))
	assert.Equal(t, 2, seq.MaxIterations) // 0 和 1
	assert.True(t, seq.Success)
	assert.Equal(t, "2+2=4", seq.FinalAnswer)
	assert.NotNil(t, seq.EndTime)
	
	// 测试摘要
	summary := seq.Summary()
	assert.Contains(t, summary, "session-1")
	assert.Contains(t, summary, "test-agent")
	assert.Contains(t, summary, "Success: true")
	assert.Contains(t, summary, "2+2=4")
}

// TestBuildSequenceIncomplete 测试未完成序列
func TestBuildSequenceIncomplete(t *testing.T) {
	store := NewInMemoryStepStore()
	ctx := context.Background()
	
	// 模拟未完成的 ReAct 循环（没有 final 步骤）
	steps := []*ReactStep{
		NewReactStep(0, StepReasoning, nil),
		NewReactStep(0, StepActing, nil).WithToolCalls([]*message.ToolUseBlock{
			{ID: "tool-1", Name: "search"},
		}),
	}
	
	for _, step := range steps {
		require.NoError(t, store.Save(ctx, step))
	}
	
	seq, err := BuildSequence(ctx, store, "session-2", "test-agent")
	require.NoError(t, err)
	assert.False(t, seq.Success)
	assert.Empty(t, seq.FinalAnswer)
	assert.Nil(t, seq.EndTime)
	
	summary := seq.Summary()
	assert.Contains(t, summary, "Success: false")
}

// TestReactStepRecorderConcurrency 测试并发记录
func TestReactStepRecorderConcurrency(t *testing.T) {
	recorder := NewReactStepRecorder(nil)
	ctx := context.Background()
	
	// 并发记录多个步骤
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			_, err := recorder.RecordReasoning(ctx, iteration, nil, nil)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()
	
	allSteps, err := recorder.GetAllSteps(ctx)
	require.NoError(t, err)
	assert.Len(t, allSteps, 10)
}
