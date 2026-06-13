package memory

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// FTSSearchResult FTS 单行搜索结果
type FTSSearchResult struct {
	MemoryID string
	Content  string
	BM25Raw  float64 // 原始 rank（越小越好）
	BM25Norm float64 // 归一化到 [0,1]（越大越好）
}

// FTSIndexV2 增强版 SQLite FTS5 全文索引，支持 trigram 分词 + CJK 短词 LIKE 回退
type FTSIndexV2 struct {
	db *sql.DB
}

// NewFTSIndexV2 打开或创建 SQLite 数据库，初始化 FTS5 表（使用 trigram 分词器）
func NewFTSIndexV2(dbPath string) (*FTSIndexV2, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	// 设置 WAL 模式提升并发写入性能
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	// 创建 FTS5 虚拟表（使用 trigram 分词器，对 CJK 更友好）
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts_v2 USING fts5(
			content,
			memory_target UNINDEXED,
			memory_type UNINDEXED,
			tokenize='trigram'
		);
	`)
	if err != nil {
		// 如果 trigram 不可用，回退到 unicode61
		_, err = db.Exec(`
			CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts_v2 USING fts5(
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
	}
	// 创建兼容旧版的 memory_fts 视图（用于测试兼容）
	_, _ = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			content,
			memory_target UNINDEXED,
			memory_type UNINDEXED,
			tokenize='unicode61'
		);
	`)
	// 创建辅助表用于 LIKE 回退搜索（存储原始内容）
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_content_aux (
			memory_id TEXT PRIMARY KEY,
			content TEXT,
			memory_target TEXT,
			memory_type TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_aux_target ON memory_content_aux(memory_target);
		CREATE INDEX IF NOT EXISTS idx_aux_type ON memory_content_aux(memory_type);
	`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &FTSIndexV2{db: db}, nil
}

// Close 关闭数据库连接
func (f *FTSIndexV2) Close() error {
	if f == nil || f.db == nil {
		return nil
	}
	return f.db.Close()
}

// Insert 插入新记忆到 FTS5 和辅助表
func (f *FTSIndexV2) Insert(node *MemoryNode) error {
	if f == nil || f.db == nil || node == nil {
		return nil
	}
	rid, err := memoryIDToInt64(node.MemoryID)
	if err != nil {
		return err
	}
	// 插入兼容旧版的 memory_fts
	_, _ = f.db.Exec(
		`INSERT INTO memory_fts(rowid, content, memory_target, memory_type) VALUES(?, ?, ?, ?)`,
		rid, segmentCJK(node.Content), node.MemoryTarget, string(node.MemoryType),
	)
	// 插入新版 memory_fts_v2
	_, err = f.db.Exec(
		`INSERT INTO memory_fts_v2(rowid, content, memory_target, memory_type) VALUES(?, ?, ?, ?)`,
		rid, sanitizeFTSQuery(node.Content), node.MemoryTarget, string(node.MemoryType),
	)
	if err != nil {
		return err
	}
	// 插入辅助表（用于 LIKE 回退）
	_, err = f.db.Exec(
		`INSERT OR REPLACE INTO memory_content_aux(memory_id, content, memory_target, memory_type) VALUES(?, ?, ?, ?)`,
		node.MemoryID, node.Content, node.MemoryTarget, string(node.MemoryType),
	)
	return err
}

// Update 更新已有记忆
func (f *FTSIndexV2) Update(node *MemoryNode) error {
	if f == nil || f.db == nil || node == nil {
		return nil
	}
	if err := f.Delete(node.MemoryID); err != nil {
		return err
	}
	return f.Insert(node)
}

// Delete 按 MemoryID 删除
func (f *FTSIndexV2) Delete(memoryID string) error {
	if f == nil || f.db == nil {
		return nil
	}
	rid, err := memoryIDToInt64(memoryID)
	if err != nil {
		return err
	}
	_, err = f.db.Exec(`DELETE FROM memory_fts_v2 WHERE rowid = ?`, rid)
	if err != nil {
		return err
	}
	// 从兼容旧版删除
	_, _ = f.db.Exec(`DELETE FROM memory_fts WHERE rowid = ?`, rid)
	_, err = f.db.Exec(`DELETE FROM memory_content_aux WHERE memory_id = ?`, memoryID)
	return err
}

// Search 全文检索，支持 CJK 短词 LIKE 回退
func (f *FTSIndexV2) Search(query string, topK int, memType *MemoryType, target string) ([]*FTSSearchResult, error) {
	if f == nil || f.db == nil {
		return nil, nil
	}
	if topK <= 0 {
		topK = 10
	}

	// 判断是否需要 LIKE 回退（CJK 短词或特殊字符）
	if shouldUseLIKEFallback(query) {
		return f.searchLikeFallback(query, topK, memType, target)
	}

	// 先尝试新版 memory_fts_v2
	segQuery := sanitizeFTSQuery(query)
	sqlStr := `SELECT rowid, content, rank FROM memory_fts_v2 WHERE memory_fts_v2 MATCH ?`
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
		// 如果新版 FTS 查询失败，回退到旧版 memory_fts
		return f.searchLegacy(query, topK, memType, target)
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
	if err := rows.Err(); err != nil {
		return f.searchLegacy(query, topK, memType, target)
	}
	// 如果新版没有结果，回退到旧版
	if len(out) == 0 {
		return f.searchLegacy(query, topK, memType, target)
	}
	return out, nil
}

// searchLegacy 使用旧版 memory_fts 搜索（兼容测试）
func (f *FTSIndexV2) searchLegacy(query string, topK int, memType *MemoryType, target string) ([]*FTSSearchResult, error) {
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
		// 如果旧版也失败，回退到 LIKE
		return f.searchLikeFallback(query, topK, memType, target)
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
	if err := rows.Err(); err != nil {
		return f.searchLikeFallback(query, topK, memType, target)
	}
	if len(out) == 0 {
		return f.searchLikeFallback(query, topK, memType, target)
	}
	return out, nil
}

// searchLikeFallback 当 FTS 无法处理时（CJK 短词、特殊字符），使用 LIKE 子串搜索
func (f *FTSIndexV2) searchLikeFallback(query string, topK int, memType *MemoryType, target string) ([]*FTSSearchResult, error) {
	likePattern := "%" + query + "%"
	sqlStr := `SELECT memory_id, content FROM memory_content_aux WHERE content LIKE ?`
	args := []any{likePattern}

	if memType != nil {
		sqlStr += ` AND memory_type = ?`
		args = append(args, string(*memType))
	}
	if target != "" {
		sqlStr += ` AND memory_target = ?`
		args = append(args, target)
	}
	sqlStr += ` LIMIT ?`
	args = append(args, topK*3) // 扩大候选集

	rows, err := f.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FTSSearchResult
	for rows.Next() {
		var mid, content string
		if err := rows.Scan(&mid, &content); err != nil {
			continue
		}
		// 计算简单相关性分数（基于匹配位置和频率）
		score := likeRelevanceScore(query, content)
		out = append(out, &FTSSearchResult{
			MemoryID: mid,
			Content:  content,
			BM25Raw:  -score, // 负值表示"越好"
			BM25Norm: score,
		})
	}
	return out, rows.Err()
}

// BM25Scores 对指定的 MemoryID 列表批量查询 BM25 分
func (f *FTSIndexV2) BM25Scores(query string, memoryIDs []string) (map[string]float64, error) {
	if f == nil || f.db == nil || len(memoryIDs) == 0 {
		return make(map[string]float64), nil
	}

	// 判断是否需要 LIKE 回退
	if shouldUseLIKEFallback(query) {
		return f.bm25ScoresLikeFallback(query, memoryIDs)
	}

	// 先尝试新版 memory_fts_v2
	segQuery := sanitizeFTSQuery(query)
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

	if len(placeholders) == 0 {
		return make(map[string]float64), nil
	}

	sqlStr := fmt.Sprintf(
		`SELECT rowid, rank FROM memory_fts_v2 WHERE memory_fts_v2 MATCH ? AND rowid IN (%s)`,
		joinPlaceholders(placeholders),
	)

	rows, err := f.db.Query(sqlStr, args...)
	if err != nil {
		// 回退到旧版
		return f.bm25ScoresLegacy(query, memoryIDs)
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
	if err := rows.Err(); err != nil {
		return f.bm25ScoresLegacy(query, memoryIDs)
	}
	// 如果新版没有结果，回退到旧版
	if len(scores) == 0 {
		return f.bm25ScoresLegacy(query, memoryIDs)
	}
	return scores, nil
}

// bm25ScoresLegacy 使用旧版 memory_fts 查询 BM25 分数
func (f *FTSIndexV2) bm25ScoresLegacy(query string, memoryIDs []string) (map[string]float64, error) {
	segQuery := segmentCJK(query)
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

	if len(placeholders) == 0 {
		return make(map[string]float64), nil
	}

	sqlStr := fmt.Sprintf(
		`SELECT rowid, rank FROM memory_fts WHERE memory_fts MATCH ? AND rowid IN (%s)`,
		joinPlaceholders(placeholders),
	)

	rows, err := f.db.Query(sqlStr, args...)
	if err != nil {
		return f.bm25ScoresLikeFallback(query, memoryIDs)
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
	if err := rows.Err(); err != nil {
		return f.bm25ScoresLikeFallback(query, memoryIDs)
	}
	if len(scores) == 0 {
		return f.bm25ScoresLikeFallback(query, memoryIDs)
	}
	return scores, nil
}

// bm25ScoresLikeFallback LIKE 回退的 BM25 分数计算
func (f *FTSIndexV2) bm25ScoresLikeFallback(query string, memoryIDs []string) (map[string]float64, error) {
	scores := make(map[string]float64, len(memoryIDs))
	if len(memoryIDs) == 0 {
		return scores, nil
	}

	placeholders := make([]string, len(memoryIDs))
	args := make([]any, len(memoryIDs))
	for i, id := range memoryIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	likePattern := "%" + query + "%"
	sqlStr := fmt.Sprintf(
		`SELECT memory_id, content FROM memory_content_aux WHERE memory_id IN (%s) AND content LIKE ?`,
		joinPlaceholders(placeholders),
	)
	args = append(args, likePattern)

	rows, err := f.db.Query(sqlStr, args...)
	if err != nil {
		return scores, err
	}
	defer rows.Close()

	for rows.Next() {
		var mid, content string
		if err := rows.Scan(&mid, &content); err != nil {
			continue
		}
		scores[mid] = likeRelevanceScore(query, content)
	}
	return scores, nil
}

// Count 返回 FTS5 表中的总记录数
func (f *FTSIndexV2) Count() (int, error) {
	if f == nil || f.db == nil {
		return 0, nil
	}
	var n int
	err := f.db.QueryRow(`SELECT COUNT(*) FROM memory_fts_v2`).Scan(&n)
	return n, err
}

// shouldUseLIKEFallback 判断是否需要使用 LIKE 回退
// CJK 短词（长度 < 3）或包含 FTS5 特殊字符时回退
func shouldUseLIKEFallback(query string) bool {
	// 检查是否全是 CJK 字符且长度 < 3
	cjkCount := 0
	totalCount := 0
	for _, r := range query {
		if isCJKRune(r) {
			cjkCount++
		}
		totalCount++
	}
	// 如果全是 CJK 且长度 < 3，使用 LIKE
	if cjkCount > 0 && cjkCount == totalCount && totalCount < 3 {
		return true
	}
	// 检查是否包含 FTS5 特殊字符
	if strings.ContainsAny(query, `"*\n\r`) {
		return true
	}
	return false
}

// sanitizeFTSQuery 移除 FTS5 特殊字符，防止注入
func sanitizeFTSQuery(query string) string {
	// FTS5 特殊字符: " * \n \r
	replacer := strings.NewReplacer(
		`"`, "",
		`*`, "",
		"\n", " ",
		"\r", " ",
	)
	return replacer.Replace(query)
}

// likeRelevanceScore 计算 LIKE 匹配的相关性分数（0-1）
func likeRelevanceScore(query, content string) float64 {
	if query == "" || content == "" {
		return 0
	}
	// 基于匹配位置和频率计算分数
	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx < 0 {
		return 0
	}
	// 位置越靠前分数越高
	posScore := 1.0 - float64(idx)/float64(len(content))
	if posScore < 0 {
		posScore = 0
	}
	// 频率加分
	freq := strings.Count(strings.ToLower(content), strings.ToLower(query))
	freqScore := math.Min(1.0, float64(freq)*0.3)
	return posScore*0.6 + freqScore*0.4
}

// --- 原有 FTSIndex 保持兼容（别名） ---

// FTSIndex 封装 SQLite FTS5 全文索引（兼容旧版）
type FTSIndex = FTSIndexV2

// NewFTSIndex 打开或创建 SQLite 数据库（兼容旧版）
func NewFTSIndex(dbPath string) (*FTSIndex, error) {
	return NewFTSIndexV2(dbPath)
}

// --- 工具函数 ---

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
	return 1.0 / (1.0 + math.Exp(rank))
}

func joinPlaceholders(p []string) string {
	return strings.Join(p, ",")
}

func isCJKRune(r rune) bool {
	return (r >= '\u4e00' && r <= '\u9fff') ||
		(r >= '\u3400' && r <= '\u4dbf') ||
		(r >= '\u3040' && r <= '\u309f') ||
		(r >= '\u30a0' && r <= '\u30ff') ||
		(r >= '\uac00' && r <= '\ud7af')
}

// segmentCJK 将连续的 CJK 字符用空格分隔（保留用于兼容）
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
