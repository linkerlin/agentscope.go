package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// ============== ToolSummarizer ==============

func TestToolSummarizer_EvaluateToolCall(t *testing.T) {
	s := NewToolSummarizer(&mockChatModel{reply: "摘要：测试摘要\n评价：质量良好\n评分：0.85"}, "zh")
	result := &ToolCallResult{ToolName: "echo", Input: map[string]any{"x": 1}, Output: "ok", Success: true, TimeCost: 1.0}
	if err := s.EvaluateToolCall(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	if result.Summary != "测试摘要" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.Score != 0.85 {
		t.Fatalf("unexpected score: %f", result.Score)
	}
}

func TestToolSummarizer_EvaluateToolCall_Nil(t *testing.T) {
	var s *ToolSummarizer
	if err := s.EvaluateToolCall(context.Background(), &ToolCallResult{}); err != nil {
		t.Fatal("expected nil")
	}
	s2 := NewToolSummarizer(nil, "zh")
	if err := s2.EvaluateToolCall(context.Background(), &ToolCallResult{}); err != nil {
		t.Fatal("expected nil")
	}
}

func TestToolSummarizer_SummarizeToolUsage(t *testing.T) {
	s := NewToolSummarizer(&mockChatModel{reply: "使用指南"}, "zh")
	results := []ToolCallResult{
		{ToolName: "echo", Input: map[string]any{"x": 1}, Summary: "s1", Evaluation: "e1", Score: 0.8, Success: true, TimeCost: 1.0, TokenCost: 10},
		{ToolName: "echo", Input: map[string]any{"x": 2}, Summary: "s2", Evaluation: "e2", Score: 0.9, Success: true, TimeCost: 2.0, TokenCost: 20},
	}
	node, err := s.SummarizeToolUsage(context.Background(), "echo", results)
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected node")
	}
	if !results[0].IsSummarized || !results[1].IsSummarized {
		t.Fatal("expected IsSummarized flag")
	}
}

func TestToolSummarizer_SummarizeToolUsage_NoModel(t *testing.T) {
	s := NewToolSummarizer(nil, "zh")
	node, err := s.SummarizeToolUsage(context.Background(), "echo", []ToolCallResult{{}})
	if err != nil || node != nil {
		t.Fatal("expected nil")
	}
}

func TestToolSummarizer_BatchEvaluate(t *testing.T) {
	s := NewToolSummarizer(&mockChatModel{reply: "摘要：x\n评价：y\n评分：0.5"}, "zh")
	results := []*ToolCallResult{{ToolName: "t1"}, {ToolName: "t2"}}
	if err := s.BatchEvaluate(context.Background(), results); err != nil {
		t.Fatal(err)
	}
}

func TestToolSummarizer_GenerateBestPractices(t *testing.T) {
	s := NewToolSummarizer(&mockChatModel{reply: "best practice"}, "zh")
	out, err := s.GenerateBestPractices(context.Background(), "echo", []ToolCallResult{{Summary: "s1"}})
	if err != nil || out != "best practice" {
		t.Fatal(err, out)
	}
}

func TestToolSummarizer_parseEvaluationResponse(t *testing.T) {
	s := NewToolSummarizer(nil, "en")
	summary, eval, score := s.parseEvaluationResponse("Summary: hello\nEvaluation: good\nScore: 0.92")
	if summary != "hello" || eval != "good" || score != 0.92 {
		t.Fatalf("unexpected: %s %s %f", summary, eval, score)
	}
}

func TestToolSummarizer_calculateStatistics(t *testing.T) {
	s := NewToolSummarizer(nil, "zh")
	stats := s.calculateStatistics([]ToolCallResult{
		{Success: true, Score: 0.8, TimeCost: 2.0, TokenCost: 20},
		{Success: false, Score: 0.6, TimeCost: 1.0, TokenCost: 10},
	})
	if stats.TotalCalls != 2 || stats.SuccessCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.SuccessRate != 0.5 {
		t.Fatalf("unexpected success rate: %f", stats.SuccessRate)
	}
}

func TestToolSummarizer_formatGuideWithStats(t *testing.T) {
	s := NewToolSummarizer(nil, "zh")
	stats := ToolStatistics{TotalCalls: 2, SuccessCount: 1, SuccessRate: 0.5, AvgScore: 0.7, AvgTimeCost: 1.5, MinTimeCost: 1.0, MaxTimeCost: 2.0, AvgTokenCost: 15, MinTokenCost: 10, MaxTokenCost: 20}
	out := s.formatGuideWithStats("guide", stats)
	if out == "" {
		t.Fatal("expected non-empty")
	}
}

func TestTruncateString(t *testing.T) {
	if truncateString("hello", 10) != "hello" {
		t.Fatal("unexpected")
	}
	if truncateString("hello world", 5) != "hello..." {
		t.Fatal("unexpected")
	}
}

// ============== PersonalSummarizer ==============

func TestPersonalSummarizer_ExtractObservations(t *testing.T) {
	s := NewPersonalSummarizer(&mockChatModel{reply: "信息：<1> <> <用户喜欢喝咖啡> <饮食偏好>\n信息：<2> <> <用户是工程师> <职业>"}, "zh")
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("我喜欢喝咖啡").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("我是一名工程师").Build(),
	}
	obs, err := s.ExtractObservations(context.Background(), msgs, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	if obs[0].Content != "用户喜欢喝咖啡" {
		t.Fatalf("unexpected content: %s", obs[0].Content)
	}
}

func TestPersonalSummarizer_ExtractObservations_Nil(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	if obs, err := s.ExtractObservations(context.Background(), nil, "u"); err != nil || obs != nil {
		t.Fatal("expected nil")
	}
	var s2 *PersonalSummarizer
	if obs, err := s2.ExtractObservations(context.Background(), []*message.Msg{{}}, "u"); err != nil || obs != nil {
		t.Fatal("expected nil")
	}
}

func TestPersonalSummarizer_ExtractInsights(t *testing.T) {
	s := NewPersonalSummarizer(&mockChatModel{reply: "洞察：<性格> <> <开朗外向>\n洞察：<职业> <> <技术专家>"}, "zh")
	obs := []*MemoryNode{NewMemoryNode(MemoryTypePersonal, "u", "喜欢交流")}
	insights, err := s.ExtractInsights(context.Background(), obs, "u")
	if err != nil {
		t.Fatal(err)
	}
	if len(insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(insights))
	}
}

func TestPersonalSummarizer_UpdateInsights(t *testing.T) {
	m := &mockChatModel{reply: "的资料：<更新后的洞察>"}
	s := NewPersonalSummarizer(m, "zh")
	insights := []*MemoryNode{
		{MemoryID: "i1", Content: "原洞察", MemoryType: MemoryTypePersonal, Metadata: map[string]any{"insight_subject": "s1"}},
	}
	observations := []*MemoryNode{
		{Content: "新观察", Metadata: map[string]any{"insight_subject": "s1"}},
	}
	updated, err := s.UpdateInsights(context.Background(), insights, observations, "u")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0].Content != "更新后的洞察" {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestPersonalSummarizer_UpdateInsights_NoRelevant(t *testing.T) {
	s := NewPersonalSummarizer(&mockChatModel{}, "zh")
	insights := []*MemoryNode{{MemoryID: "i1", Content: "old", MemoryType: MemoryTypePersonal}}
	observations := []*MemoryNode{{Content: "new", Metadata: map[string]any{"insight_subject": "other"}}}
	updated, err := s.UpdateInsights(context.Background(), insights, observations, "u")
	if err != nil || len(updated) != 1 || updated[0].Content != "old" {
		t.Fatal("expected unchanged")
	}
}

func TestPersonalSummarizer_HandleContraRepeat(t *testing.T) {
	s := NewPersonalSummarizer(&mockChatModel{reply: "<1> <被包含>\n<2> <无>"}, "zh")
	memories := []*MemoryNode{
		{MemoryID: "m1", Content: "用户喜欢喝咖啡"},
		{MemoryID: "m2", Content: "用户喜欢喝咖啡和茶"},
	}
	kept, removed, err := s.HandleContraRepeat(context.Background(), memories)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || len(removed) != 1 {
		t.Fatalf("unexpected kept=%d removed=%d", len(kept), len(removed))
	}
}

func TestPersonalSummarizer_HandleContraRepeat_Short(t *testing.T) {
	s := NewPersonalSummarizer(&mockChatModel{}, "zh")
	kept, removed, err := s.HandleContraRepeat(context.Background(), []*MemoryNode{{MemoryID: "m1"}})
	if err != nil || len(kept) != 1 || len(removed) != 0 {
		t.Fatal("expected unchanged")
	}
}

func TestPersonalSummarizer_filterTimeRelatedMessages(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("今天天气很好").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("我喜欢咖啡").Build(),
	}
	filtered := s.filterTimeRelatedMessages(msgs)
	if len(filtered) != 1 || filtered[0].GetTextContent() != "我喜欢咖啡" {
		t.Fatalf("unexpected filtered: %v", filtered)
	}
}

func TestPersonalSummarizer_parseObservations(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	msgs := []*message.Msg{message.NewMsg().TextContent("msg1").Build()}
	nodes := s.parseObservations("信息：<1> <> <内容> <关键词>", msgs, "u")
	if len(nodes) != 1 || nodes[0].Content != "内容" {
		t.Fatalf("unexpected: %+v", nodes)
	}
}

func TestPersonalSummarizer_parseInsights(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	nodes := s.parseInsights("洞察：<主题> <> <描述内容>", "u")
	if len(nodes) != 1 || nodes[0].Content != "描述内容" {
		t.Fatalf("unexpected: %+v", nodes)
	}
}

func TestPersonalSummarizer_parseContraRepeatResponse(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	memories := []*MemoryNode{{MemoryID: "m1"}, {MemoryID: "m2"}}
	kept, removed, err := s.parseContraRepeatResponse("<1> <矛盾>\n<2> <无>", memories)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || len(removed) != 1 {
		t.Fatalf("unexpected kept=%d removed=%d", len(kept), len(removed))
	}
}

func TestPersonalSummarizer_findRelevantObservations(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	insight := &MemoryNode{Content: "喜欢咖啡", Metadata: map[string]any{"insight_subject": "饮食"}}
	obs := []*MemoryNode{
		{Content: "喜欢咖啡", Metadata: map[string]any{"insight_subject": "饮食"}},
		{Content: "喜欢跑步", Metadata: map[string]any{"insight_subject": "运动"}},
	}
	rel := s.findRelevantObservations(insight, obs)
	if len(rel) != 1 {
		t.Fatalf("expected 1 relevant, got %d", len(rel))
	}
}

func TestPersonalSummarizer_extractKeywords(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	kws := s.extractKeywords("Hello world test")
	if len(kws) != 3 { // hello, world, test
		t.Fatalf("unexpected keywords: %v", kws)
	}
}

func TestPersonalSummarizer_calculateJaccardSimilarity(t *testing.T) {
	s := NewPersonalSummarizer(nil, "zh")
	s1 := map[string]struct{}{"a": {}, "b": {}}
	s2 := map[string]struct{}{"b": {}, "c": {}}
	if s.calculateJaccardSimilarity(s1, s2) != 1.0/3.0 {
		t.Fatal("unexpected similarity")
	}
}

// ============== ProceduralSummarizer ==============

func TestProceduralSummarizer_ExtractFromTrajectories(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: `[{"when_to_use":"场景","memory":"经验"}]`}, "zh")
	trajs := []Trajectory{
		{Messages: []*message.Msg{message.NewMsg().TextContent("step1").Build()}, Score: 0.95, TaskName: "task1"},
	}
	memories, err := s.ExtractFromTrajectories(context.Background(), trajs)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
}

func TestProceduralSummarizer_ExtractFromSingleTrajectory_Success(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: `[{"when_to_use":"s","memory":"m"}]`}, "zh")
	traj := Trajectory{Messages: []*message.Msg{message.NewMsg().TextContent("ok").Build()}, Score: 0.95, TaskName: "t1"}
	memories, err := s.ExtractFromSingleTrajectory(context.Background(), traj)
	if err != nil || len(memories) != 1 {
		t.Fatal(err, len(memories))
	}
	if memories[0].Metadata["pattern_type"] != "success" {
		t.Fatalf("unexpected pattern_type: %v", memories[0].Metadata["pattern_type"])
	}
}

func TestProceduralSummarizer_ExtractFromSingleTrajectory_Failure(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: `[{"when_to_use":"s","memory":"m"}]`}, "zh")
	traj := Trajectory{Messages: []*message.Msg{message.NewMsg().TextContent("fail").Build()}, Score: 0.3, TaskName: "t1"}
	memories, err := s.ExtractFromSingleTrajectory(context.Background(), traj)
	if err != nil || len(memories) != 1 {
		t.Fatal(err, len(memories))
	}
	if memories[0].Metadata["pattern_type"] != "failure" {
		t.Fatalf("unexpected pattern_type: %v", memories[0].Metadata["pattern_type"])
	}
}

func TestProceduralSummarizer_ExtractFromSingleTrajectory_Mid(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: `[{"when_to_use":"s","memory":"m"}]`}, "zh")
	traj := Trajectory{Messages: []*message.Msg{message.NewMsg().TextContent("mid").Build()}, Score: 0.7, TaskName: "t1"}
	memories, err := s.ExtractFromSingleTrajectory(context.Background(), traj)
	if err != nil || len(memories) != 1 {
		t.Fatal(err, len(memories))
	}
	if memories[0].Metadata["pattern_type"] != "improvement" {
		t.Fatalf("unexpected pattern_type: %v", memories[0].Metadata["pattern_type"])
	}
}

func TestProceduralSummarizer_ExtractSuccessPattern(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: `[{"when_to_use":"s","memory":"success pattern"}]`}, "zh")
	memories, err := s.ExtractSuccessPattern(context.Background(), []Trajectory{{TaskName: "t1", Messages: []*message.Msg{{}}}})
	if err != nil || len(memories) != 1 {
		t.Fatal(err, len(memories))
	}
}

func TestProceduralSummarizer_ExtractFailureLesson(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: `[{"when_to_use":"s","memory":"lesson"}]`}, "zh")
	memories, err := s.ExtractFailureLesson(context.Background(), []Trajectory{{TaskName: "t1", Messages: []*message.Msg{{}}}})
	if err != nil || len(memories) != 1 {
		t.Fatal(err, len(memories))
	}
}

func TestProceduralSummarizer_ValidateMemories(t *testing.T) {
	s := NewProceduralSummarizer(&mockChatModel{reply: "<1> <有效>\n<2> <无效>"}, "zh")
	memories := []*MemoryNode{{Content: "m1"}, {Content: "m2"}}
	valid, invalid := s.ValidateMemories(context.Background(), memories, Trajectory{})
	if len(valid) != 1 || len(invalid) != 1 {
		t.Fatalf("unexpected valid=%d invalid=%d", len(valid), len(invalid))
	}
}

func TestProceduralSummarizer_DeduplicateMemories(t *testing.T) {
	s := NewProceduralSummarizer(nil, "zh")
	memories := []*MemoryNode{
		{Content: "hello world"},
		{Content: "hello world"},
		{Content: "完全不同的内容"},
	}
	unique := s.DeduplicateMemories(memories)
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique, got %d", len(unique))
	}
}

func TestProceduralSummarizer_parseTaskMemoriesFromText(t *testing.T) {
	s := NewProceduralSummarizer(nil, "zh")
	nodes := s.parseTaskMemoriesFromText("经验：<场景A> <> <内容A>", Trajectory{TaskName: "t1"})
	if len(nodes) != 1 || nodes[0].Content != "内容A" {
		t.Fatalf("unexpected: %+v", nodes)
	}
}

func TestProceduralSummarizer_parseValidationResponse(t *testing.T) {
	s := NewProceduralSummarizer(nil, "zh")
	memories := []*MemoryNode{{Content: "m1"}, {Content: "m2"}}
	valid, invalid := s.parseValidationResponse("<1> <valid>\n<2> <invalid>", memories)
	if len(valid) != 1 || len(invalid) != 1 {
		t.Fatalf("unexpected valid=%d invalid=%d", len(valid), len(invalid))
	}
}

func TestProceduralSummarizer_extractJSONFromMarkdown(t *testing.T) {
	s := NewProceduralSummarizer(nil, "zh")
	out := s.extractJSONFromMarkdown("```json\n{\"a\":1}\n```")
	if out != "{\"a\":1}" {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestProceduralSummarizer_calculateContentSimilarity(t *testing.T) {
	s := NewProceduralSummarizer(nil, "zh")
	if s.calculateContentSimilarity("hello world", "hello world") != 1.0 {
		t.Fatal("expected 1.0 for identical")
	}
	if s.calculateContentSimilarity("", "hello") != 0.0 {
		t.Fatal("expected 0.0 for empty")
	}
}

func TestProceduralSummarizer_tokenize(t *testing.T) {
	s := NewProceduralSummarizer(nil, "zh")
	words := s.tokenize("Hello, world!")
	if len(words) != 2 {
		t.Fatalf("unexpected tokens: %v", words)
	}
}
