package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFTSIndexCRUD(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewFTSIndex(filepath.Join(dir, "reme.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	n1 := NewMemoryNode(MemoryTypePersonal, "alice", "用户喜欢 Go 语言")
	if err := idx.Insert(n1); err != nil {
		t.Fatal(err)
	}

	cnt, err := idx.Count()
	if err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("expected count 1, got %d", cnt)
	}

	// Update
	n1.Content = "用户热爱 Go 语言编程"
	if err := idx.Update(n1); err != nil {
		t.Fatal(err)
	}

	res, err := idx.Search("Go", 5, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	// Content 在插入时被 segmentCJK 处理过，不再与原始字符串完全一致
	if res[0].Content != segmentCJK("用户热爱 Go 语言编程") {
		t.Fatalf("unexpected search content: %q", res[0].Content)
	}

	// Delete
	if err := idx.Delete(n1.MemoryID); err != nil {
		t.Fatal(err)
	}
	cnt2, _ := idx.Count()
	if cnt2 != 0 {
		t.Fatalf("expected count 0 after delete, got %d", cnt2)
	}
}

func TestFTSIndexSearchRanking(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewFTSIndex(filepath.Join(dir, "reme.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	docs := []string{
		"Go 语言入门教程",
		"Python 数据分析",
		"Go 语言高级编程与并发",
	}
	for i, content := range docs {
		n := NewMemoryNode(MemoryTypePersonal, "u"+string(rune('0'+i)), content)
		_ = idx.Insert(n)
	}

	res, err := idx.Search("Go 语言", 5, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	// BM25 rank 越小匹配度越高
	if res[0].BM25Raw > res[1].BM25Raw {
		t.Fatalf("expected ascending rank order, got %v then %v", res[0].BM25Raw, res[1].BM25Raw)
	}
	// BM25Norm 越大匹配度越高
	if res[0].BM25Norm < res[1].BM25Norm {
		t.Fatalf("expected descending norm order, got %v then %v", res[0].BM25Norm, res[1].BM25Norm)
	}
}

func TestFTSIndexBM25Scores(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewFTSIndex(filepath.Join(dir, "reme.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	n1 := NewMemoryNode(MemoryTypePersonal, "alice", "Go 语言并发模式")
	n2 := NewMemoryNode(MemoryTypePersonal, "bob", "Python 机器学习")
	_ = idx.Insert(n1)
	_ = idx.Insert(n2)

	scores, err := idx.BM25Scores("Go 并发", []string{n1.MemoryID, n2.MemoryID, "nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if scores[n1.MemoryID] <= 0 {
		t.Fatalf("expected positive score for n1, got %v", scores[n1.MemoryID])
	}
	if scores[n2.MemoryID] > 0.1 {
		// n2 与查询几乎无关，分数应极低
		t.Fatalf("expected near-zero score for n2, got %v", scores[n2.MemoryID])
	}
}

func TestFTSIndexMemoryIDRoundTrip(t *testing.T) {
	id := "a1b2c3d4e5f67890"
	v, err := memoryIDToInt64(id)
	if err != nil {
		t.Fatal(err)
	}
	back := int64ToMemoryID(v)
	if back != id {
		t.Fatalf("roundtrip failed: %s -> %d -> %s", id, v, back)
	}
}

func TestBM25Normalize(t *testing.T) {
	// rank 越负，匹配度越高，norm 越接近 1
	if bm25Normalize(-10) < 0.99 {
		t.Fatalf("expected near 1 for rank=-10, got %v", bm25Normalize(-10))
	}
	if bm25Normalize(0) < 0.49 || bm25Normalize(0) > 0.51 {
		t.Fatalf("expected ~0.5 for rank=0, got %v", bm25Normalize(0))
	}
	if bm25Normalize(10) > 0.01 {
		t.Fatalf("expected near 0 for rank=10, got %v", bm25Normalize(10))
	}
}

func TestFTSIndexDBFileCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".agentscope", "reme.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	idx, err := NewFTSIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("db file not created")
	}
}


func TestFTSIndexNilClose(t *testing.T) {
	var idx *FTSIndex
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFTSIndexNilOperations(t *testing.T) {
	var idx *FTSIndex
	// Nil receiver returns nil for most operations
	if err := idx.Insert(nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.Update(nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.Delete(""); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Search("x", 1, nil, ""); err != nil {
		t.Fatal(err)
	}
}

func TestFTSIndexSearchWithFilter(t *testing.T) {
	dir := t.TempDir()
	idx, _ := NewFTSIndex(filepath.Join(dir, "reme.db"))
	defer idx.Close()

	n1 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢 Go")
	n2 := NewMemoryNode(MemoryTypePersonal, "bob", "喜欢 Python")
	_ = idx.Insert(n1)
	_ = idx.Insert(n2)

	mt := MemoryTypePersonal
	res, err := idx.Search("喜欢", 5, &mt, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].MemoryID != n1.MemoryID {
		t.Fatalf("unexpected filter result: %v", res)
	}
}
