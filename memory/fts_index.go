package memory

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// FTSIndex 封装 SQLite FTS5 全文索引
type FTSIndex struct {
	db *sql.DB
}

// FTSSearchResult FTS 单行搜索结果
type FTSSearchResult struct {
	MemoryID string
	Content  string
	BM25Raw  float64 // 原始 rank（越小越好）
	BM25Norm float64 // 归一化到 [0,1]（越大越好）
}

// NewFTSIndex 打开或创建 SQLite 数据库，初始化 FTS5 表
func NewFTSIndex(dbPath string) (*FTSIndex, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	// 设置 WAL 模式提升并发写入性能
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	// 创建 FTS5 虚拟表
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			content,
			memory_target UNINDEXED,
			memory_type UNINDEXED,
			tokenize='unicode61'
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &FTSIndex{db: db}, nil
}

// Close 关闭数据库连接
func (f *FTSIndex) Close() error {
	if f == nil || f.db == nil {
		return nil
	}
	return f.db.Close()
}

// Insert 插入新记忆到 FTS5
func (f *FTSIndex) Insert(node *MemoryNode) error {
	if f == nil || f.db == nil || node == nil {
		return nil
	}
	rid, err := memoryIDToInt64(node.MemoryID)
	if err != nil {
		return err
	}
	_, err = f.db.Exec(
		`INSERT INTO memory_fts(rowid, content, memory_target, memory_type) VALUES(?, ?, ?, ?)`,
		rid, segmentCJK(node.Content), node.MemoryTarget, string(node.MemoryType),
	)
	return err
}

// Update 更新已有记忆的 content（FTS5 不支持 UPDATE rowid，需先 Delete 再 Insert）
func (f *FTSIndex) Update(node *MemoryNode) error {
	if f == nil || f.db == nil || node == nil {
		return nil
	}
	if err := f.Delete(node.MemoryID); err != nil {
		return err
	}
	return f.Insert(node)
}

// Delete 按 MemoryID 删除
func (f *FTSIndex) Delete(memoryID string) error {
	if f == nil || f.db == nil {
		return nil
	}
	rid, err := memoryIDToInt64(memoryID)
	if err != nil {
		return err
	}
	_, err = f.db.Exec(`DELETE FROM memory_fts WHERE rowid = ?`, rid)
	return err
}

// Search 全文检索，返回按 BM25 排序的结果
func (f *FTSIndex) Search(query string, topK int, memType *MemoryType, target string) ([]*FTSSearchResult, error) {
	if f == nil || f.db == nil {
		return nil, nil
	}
	if topK <= 0 {
		topK = 10
	}

	segQuery := segmentCJK(query)
	sqlStr := `SELECT rowid, content, rank FROM memory_fts WHERE memory_fts MATCH ?`
	args := []any{segQuery}

	if memType != nil {
		sqlStr += ` AND memory_type = ?`
		args = append(args, string(*memType))
	}
	if target != "" {
		sqlStr += ` AND memory_target = ?`
		args = append(args, target)
	}
	sqlStr += ` ORDER BY rank LIMIT ?`
	args = append(args, topK)

	rows, err := f.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FTSSearchResult
	for rows.Next() {
		var rid int64
		var content string
		var rank float64
		if err := rows.Scan(&rid, &content, &rank); err != nil {
			continue
		}
		out = append(out, &FTSSearchResult{
			MemoryID: int64ToMemoryID(rid),
			Content:  content,
			BM25Raw:  rank,
			BM25Norm: bm25Normalize(rank),
		})
	}
	return out, rows.Err()
}

// BM25Scores 对指定的 MemoryID 列表批量查询 BM25 分（用于混合重排第二阶段）
func (f *FTSIndex) BM25Scores(query string, memoryIDs []string) (map[string]float64, error) {
	if f == nil || f.db == nil || len(memoryIDs) == 0 {
		return make(map[string]float64), nil
	}

	segQuery := segmentCJK(query)
	// 构建 IN 子句，只收集有效 ID
	var placeholders []string
	args := make([]any, 0, len(memoryIDs)+1)
	args = append(args, segQuery)
	for _, id := range memoryIDs {
		v, err := memoryIDToInt64(id)
		if err != nil {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, v)
	}

	// 如果没有任何有效 ID，直接返回空
	if len(placeholders) == 0 {
		return make(map[string]float64), nil
	}

	sqlStr := fmt.Sprintf(
		`SELECT rowid, rank FROM memory_fts WHERE memory_fts MATCH ? AND rowid IN (%s)`,
		joinPlaceholders(placeholders),
	)

	rows, err := f.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scores := make(map[string]float64, len(memoryIDs))
	for rows.Next() {
		var rid int64
		var rank float64
		if err := rows.Scan(&rid, &rank); err != nil {
			continue
		}
		scores[int64ToMemoryID(rid)] = bm25Normalize(rank)
	}
	return scores, rows.Err()
}

// Count 返回 FTS5 表中的总记录数（调试用）
func (f *FTSIndex) Count() (int, error) {
	if f == nil || f.db == nil {
		return 0, nil
	}
	var n int
	err := f.db.QueryRow(`SELECT COUNT(*) FROM memory_fts`).Scan(&n)
	return n, err
}

// memoryID 是 16 位 hex，可直接映射为 int64
func memoryIDToInt64(id string) (int64, error) {
	if len(id) > 16 {
		id = id[:16]
	}
	u, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return 0, err
	}
	return int64(u), nil
}

func int64ToMemoryID(v int64) string {
	return fmt.Sprintf("%016x", uint64(v))
}

func bm25Normalize(rank float64) float64 {
	// 当 rank 越负，BM25 匹配度越高，结果越接近 1
	return 1.0 / (1.0 + math.Exp(rank))
}

func joinPlaceholders(p []string) string {
	return strings.Join(p, ",")
}

// segmentCJK 将连续的 CJK 字符用空格分隔，使 unicode61 tokenizer 能逐字索引
func segmentCJK(text string) string {
	var out []rune
	var prevCJK bool
	for _, r := range text {
		isCJK := isCJKRune(r)
		if isCJK && prevCJK {
			out = append(out, ' ')
		}
		out = append(out, r)
		prevCJK = isCJK
	}
	return string(out)
}

func isCJKRune(r rune) bool {
	return (r >= '\u4e00' && r <= '\u9fff') ||
		(r >= '\u3400' && r <= '\u4dbf') ||
		(r >= '\u3040' && r <= '\u309f') ||
		(r >= '\u30a0' && r <= '\u30ff') ||
		(r >= '\uac00' && r <= '\ud7af')
}
