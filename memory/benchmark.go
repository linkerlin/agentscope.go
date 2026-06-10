package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// Benchmark 记忆系统基准评估接口
type Benchmark interface {
	Name() string
	Run(ctx context.Context, mem VectorMemory) (*BenchmarkResult, error)
}

// BenchmarkResult 基准评估结果
type BenchmarkResult struct {
	Name            string             `json:"name"`
	OverallScore    float64            `json:"overall_score"`
	MemoryAccuracy  float64            `json:"memory_accuracy"`
	QAAccuracy      float64            `json:"qa_accuracy"`
	DetailScores    map[string]float64 `json:"detail_scores"`
	TotalTime       time.Duration      `json:"total_time"`
	MemoryCount     int                `json:"memory_count"`
	Notes           []string           `json:"notes"`
}

// HaluMemBenchmark 幻觉检测基准
type HaluMemBenchmark struct {
	TestCases []HaluMemTestCase
}

// HaluMemTestCase 幻觉检测测试用例
type HaluMemTestCase struct {
	Conversation    []*message.Msg
	GroundTruth     []string
	Hallucination   []string
	Queries         []string
	ExpectedAnswers []string
}

func (b *HaluMemBenchmark) Name() string {
	return "HaluMem"
}

func (b *HaluMemBenchmark) Run(ctx context.Context, mem VectorMemory) (*BenchmarkResult, error) {
	start := time.Now()
	result := &BenchmarkResult{
		Name:         b.Name(),
		DetailScores: make(map[string]float64),
	}

	if mem == nil {
		return result, fmt.Errorf("benchmark: VectorMemory is nil")
	}

	var totalQuestions, correctAnswers int
	var totalMemories, accurateMemories int
	sessionID := "haluMemSession"

	for _, tc := range b.TestCases {
		for _, msg := range tc.Conversation {
			_ = mem.Add(msg)
		}

		_ = mem.SaveTo(sessionID)

		for _, query := range tc.Queries {
			nodes, err := mem.RetrieveMemory(ctx, query, RetrieveOptions{
				TopK:     5,
				MinScore: 0.3,
			})
			if err != nil {
				continue
			}
			totalQuestions++
			totalMemories += len(nodes)

			for _, node := range nodes {
				for _, truth := range tc.GroundTruth {
					if node.Content == truth {
						accurateMemories++
					}
				}
				for _, halluc := range tc.Hallucination {
					if node.Content == halluc {
						accurateMemories--
					}
				}
			}
			correctAnswers++
		}

		_ = mem.Clear()
	}

	if totalQuestions > 0 {
		result.QAAccuracy = float64(correctAnswers) / float64(totalQuestions)
	}
	if totalMemories > 0 {
		result.MemoryAccuracy = float64(accurateMemories) / float64(totalMemories)
		if result.MemoryAccuracy < 0 {
			result.MemoryAccuracy = 0
		}
	}
	result.OverallScore = (result.MemoryAccuracy + result.QAAccuracy) / 2
	result.TotalTime = time.Since(start)
	result.MemoryCount = totalMemories

	return result, nil
}

// LoCoMoBenchmark 长对话记忆基准
type LoCoMoBenchmark struct {
	TestConversations  [][]*message.Msg
	TestQueries        []string
	ExpectedRetrieval  []string
}

func (b *LoCoMoBenchmark) Name() string {
	return "LoCoMo"
}

func (b *LoCoMoBenchmark) Run(ctx context.Context, mem VectorMemory) (*BenchmarkResult, error) {
	start := time.Now()
	result := &BenchmarkResult{
		Name:         b.Name(),
		DetailScores: make(map[string]float64),
	}

	if mem == nil {
		return result, fmt.Errorf("benchmark: VectorMemory is nil")
	}

	for _, conversation := range b.TestConversations {
		for _, msg := range conversation {
			_ = mem.Add(msg)
		}

		sessionID := fmt.Sprintf("locomo_%d", time.Now().UnixNano())
		_ = mem.SaveTo(sessionID)
		_ = mem.LoadFrom(sessionID)

		// 压缩对话
		compactMessages := make([]*message.Msg, 0, len(conversation))
		for _, m := range conversation {
			if len(compactMessages) >= 20 {
				break
			}
			compactMessages = append(compactMessages, m)
		}
		if len(compactMessages) > 0 {
			summary, err := mem.CompactMemory(ctx, compactMessages, CompactOptions{
				CompactRatio:  0.2,
				ReserveTokens: 512,
			})
			if err == nil && summary != "" {
				_ = mem.Add(message.NewMsg().Role(message.RoleSystem).TextContent(summary).Build())
			}
		}
	}

	var total, correct int
	for i, query := range b.TestQueries {
		nodes, err := mem.RetrieveMemory(ctx, query, RetrieveOptions{
			TopK:     10,
			MinScore: 0.1,
		})
		if err != nil {
			continue
		}
		total++
		for _, node := range nodes {
			if i < len(b.ExpectedRetrieval) && node.Content == b.ExpectedRetrieval[i] {
				correct++
				break
			}
		}
	}

	if total > 0 {
		result.MemoryAccuracy = float64(correct) / float64(total)
	}
	result.OverallScore = result.MemoryAccuracy
	result.TotalTime = time.Since(start)

	return result, nil
}

// RunBenchmarkSuite 运行一组基准测试
func RunBenchmarkSuite(ctx context.Context, mem VectorMemory, benchmarks ...Benchmark) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult
	for _, b := range benchmarks {
		r, err := b.Run(ctx, mem)
		if err != nil {
			return results, fmt.Errorf("benchmark %q: %w", b.Name(), err)
		}
		results = append(results, r)
	}
	return results, nil
}
