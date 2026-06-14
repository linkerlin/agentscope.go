package memory

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/embedding/onnx"
	"github.com/linkerlin/agentscope.go/message"
)

// benchEmbed 固定维度嵌入模型（用于基准测试）
type benchEmbed struct {
	dim int
}

func (e benchEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, e.dim)
	seed := 0
	for _, c := range text {
		seed += int(c)
	}
	r := rand.New(rand.NewSource(int64(seed)))
	for i := range vec {
		vec[i] = r.Float32()
	}
	return vec, nil
}

func (e benchEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		v, _ := e.Embed(ctx, text)
		results[i] = v
	}
	return results, nil
}

// BenchmarkVectorStoreLocal_Insert 基准测试本地向量存储插入
func BenchmarkVectorStoreLocal_Insert(b *testing.B) {
	ctx := context.Background()
	embed := benchEmbed{dim: 128}
	store := NewLocalVectorStore(embed)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := &MemoryNode{
			MemoryID: fmt.Sprintf("bench-%d", i),
			Content:  fmt.Sprintf("message %d", i),
			Vector:   nil,
		}
		_ = store.Insert(ctx, []*MemoryNode{node})
	}
}

// BenchmarkVectorStoreLocal_Search 基准测试本地向量存储搜索
func BenchmarkVectorStoreLocal_Search(b *testing.B) {
	ctx := context.Background()
	embed := benchEmbed{dim: 128}
	store := NewLocalVectorStore(embed)

	for i := 0; i < 1000; i++ {
		node := &MemoryNode{
			MemoryID: fmt.Sprintf("bench-%d", i),
			Content:  fmt.Sprintf("document %d", i),
		}
		_ = store.Insert(ctx, []*MemoryNode{node})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Search(ctx, "document", RetrieveOptions{TopK: 10})
	}
}

// BenchmarkVectorStoreLocal_Search_HNSW 基准测试 HNSW 索引搜索
// 注意：HNSW 索引当前有并发问题，此基准测试暂时跳过
func BenchmarkVectorStoreLocal_Search_HNSW(b *testing.B) {
	b.Skip("HNSW index has concurrency issues - skipping")
}

// BenchmarkFTSIndex_Search_Large 基准测试 FTS 搜索
func BenchmarkFTSIndex_Search_Large(b *testing.B) {
	dir := b.TempDir()
	idx, err := NewFTSIndex(dir + "/fts.db")
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()

	for i := 0; i < 1000; i++ {
		_ = idx.Insert(NewMemoryNode(MemoryTypePersonal, "u", fmt.Sprintf("doc number %d content", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Search("content", 10, nil, "")
	}
}

// BenchmarkReMeFileMemory_Add 基准测试 ReMe 文件记忆添加
func BenchmarkReMeFileMemory_Add(b *testing.B) {
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = b.TempDir()
	mem, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer mem.Close()

	msg := message.NewMsg().Role(message.RoleUser).TextContent("benchmark message").Build()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mem.Add(msg)
	}
}

// BenchmarkReMeVectorMemory_Retrieve 基准测试 ReMe 向量记忆检索
func BenchmarkReMeVectorMemory_Retrieve(b *testing.B) {
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = b.TempDir()
	embed := benchEmbed{dim: 128}
	store := NewLocalVectorStore(embed)

	mem, _ := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), store, embed)

	for i := 0; i < 100; i++ {
		node := NewMemoryNode(MemoryTypePersonal, "u", fmt.Sprintf("memory about %d", i))
		_ = mem.AddMemory(context.Background(), node)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mem.RetrieveMemory(context.Background(), "memory", RetrieveOptions{TopK: 5})
	}
}

// BenchmarkEmbeddingCache_Hit 基准测试缓存命中
func BenchmarkEmbeddingCache_Hit(b *testing.B) {
	base := &countingEmbedder{v: []float32{0.1, 0.2, 0.3}}
	cache := NewEmbeddingCache(base, 1024)
	ctx := context.Background()
	_, _ = cache.Embed(ctx, "hello")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Embed(ctx, "hello")
	}
}

// BenchmarkEmbeddingCache_Miss 基准测试缓存未命中
func BenchmarkEmbeddingCache_Miss(b *testing.B) {
	base := &countingEmbedder{v: []float32{0.1, 0.2, 0.3}}
	cache := NewEmbeddingCache(base, 1024)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Embed(ctx, fmt.Sprintf("hello %d", i))
	}
}

// BenchmarkEmbeddingCache_Concurrent 基准测试并发缓存访问
func BenchmarkEmbeddingCache_Concurrent(b *testing.B) {
	base := &countingEmbedder{v: []float32{0.1, 0.2, 0.3}}
	cache := NewEmbeddingCache(base, 1024)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		_, _ = cache.Embed(ctx, fmt.Sprintf("key-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = cache.Embed(ctx, fmt.Sprintf("key-%d", i%100))
			i++
		}
	})
}

// BenchmarkSummarizer_Summarize 基准测试摘要生成
func BenchmarkSummarizer_Summarize(b *testing.B) {
	s := &Summarizer{WorkingDir: b.TempDir()}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.AppendToMemoryMD(fmt.Sprintf("Title %d", i), fmt.Sprintf("Body %d", i))
	}
}

// BenchmarkMemoryCollector_Run 基准测试内存 GC
func BenchmarkMemoryCollector_Run(b *testing.B) {
	embed := benchEmbed{dim: 128}
	store := NewLocalVectorStore(embed)
	collector := NewMemoryCollector(store)

	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		node := NewMemoryNode(MemoryTypePersonal, "u", fmt.Sprintf("old memory %d", i))
		node.Metadata = map[string]any{
			"freq":          1,
			"last_accessed": time.Now().Add(-720 * time.Hour).Unix(),
			"utility":       0.1,
		}
		_ = store.Insert(ctx, []*MemoryNode{node})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = collector.Collect(ctx)
	}
}

// BenchmarkONNXImagePreprocess 基准测试 ONNX 图像预处理
func BenchmarkONNXImagePreprocess(b *testing.B) {
	preprocessor := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())

	testData := make([]byte, 1024*768*3)
	for i := range testData {
		testData[i] = byte(rand.Intn(256))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.Preprocess(bytes.NewReader(testData))
	}
}

// BenchmarkONNXAudioPreprocess 基准测试 ONNX 音频预处理
func BenchmarkONNXAudioPreprocess(b *testing.B) {
	preprocessor := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())

	samples := make([]float32, 160000)
	for i := range samples {
		samples[i] = rand.Float32()*2 - 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.Preprocess(samples, 16000)
	}
}

// BenchmarkCrossModalSimilarity 基准测试跨模态相似度
func BenchmarkCrossModalSimilarity(b *testing.B) {
	embedding1 := make([]float32, 512)
	embedding2 := make([]float32, 512)
	for i := range embedding1 {
		embedding1[i] = rand.Float32()
		embedding2[i] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		onnx.CrossModalSimilarity(embedding1, embedding2)
	}
}

// BenchmarkVectorStoreComparison 比较不同规模下的向量存储性能
func BenchmarkVectorStoreComparison(b *testing.B) {
	embed := benchEmbed{dim: 128}

	b.Run("Small_100", func(b *testing.B) {
		ctx := context.Background()
		store := NewLocalVectorStore(embed)
		for i := 0; i < 100; i++ {
			node := &MemoryNode{
				MemoryID: fmt.Sprintf("node-%d", i),
				Content:  fmt.Sprintf("content %d", i),
			}
			_ = store.Insert(ctx, []*MemoryNode{node})
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = store.Search(ctx, "content", RetrieveOptions{TopK: 10})
		}
	})

	b.Run("Medium_1000", func(b *testing.B) {
		ctx := context.Background()
		store := NewLocalVectorStore(embed)
		for i := 0; i < 1000; i++ {
			node := &MemoryNode{
				MemoryID: fmt.Sprintf("node-%d", i),
				Content:  fmt.Sprintf("content %d", i),
			}
			_ = store.Insert(ctx, []*MemoryNode{node})
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = store.Search(ctx, "content", RetrieveOptions{TopK: 10})
		}
	})

	b.Run("Large_10000_HNSW", func(b *testing.B) {
		b.Skip("HNSW index has concurrency issues - skipping")
	})
}

// BenchmarkReActOrchestrator_InjectMemory 基准测试 ReAct 记忆注入
func BenchmarkReActOrchestrator_InjectMemory(b *testing.B) {
	ctx := context.Background()
	dir := b.TempDir()

	fts, _ := NewFTSIndex(dir + "/fts.db")
	defer fts.Close()

	embed := benchEmbed{dim: 128}
	store := NewLocalVectorStore(embed)

	for i := 0; i < 50; i++ {
		node := &MemoryNode{
			MemoryID: fmt.Sprintf("mem-%d", i),
			Content:  fmt.Sprintf("knowledge about %d", i),
		}
		_ = store.Insert(ctx, []*MemoryNode{node})
		_ = fts.Insert(NewMemoryNode(MemoryTypePersonal, "u", node.Content))
	}

	recorder := NewReactStepRecorder(NewInMemoryStepStore())
	orchestrator := NewReactOrchestrator(recorder, store, DefaultReactOrchestratorConfig())

	query := "knowledge"
	history := []*message.Msg{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = orchestrator.InjectMemory(ctx, query, history, "user", "task")
	}
}

// BenchmarkAsyncTaskQueue_Process 基准测试异步任务队列
func BenchmarkAsyncTaskQueue_Process(b *testing.B) {
	queue := NewAsyncTaskQueue(4)
	queue.RegisterHandler(TaskTypeSummarize, func(ctx context.Context, task *AsyncTask) (*AsyncTaskResult, error) {
		time.Sleep(1 * time.Millisecond)
		return &AsyncTaskResult{TaskID: task.ID, Success: true}, nil
	})
	defer queue.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.SubmitSummarize(fmt.Sprintf("memory-%d", i), "content", 5)
	}

	// 等待所有任务完成
	for {
		stats := queue.Stats()
		if stats["pending"].(int) == 0 && stats["running"].(int) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
