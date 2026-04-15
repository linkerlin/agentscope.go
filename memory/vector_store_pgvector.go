package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pgvector/pgvector-go"
)

// PGVectorStore 基于 PostgreSQL + pgvector 的远程向量存储实现
type PGVectorStore struct {
	db     *sql.DB
	table  string
	embed  EmbeddingModel
	dim    int
}

// NewPGVectorStore 创建 PGVector 向量存储
func NewPGVectorStore(connStr, table string, dim int, embed EmbeddingModel) (*PGVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}
	s := &PGVectorStore{
		db:    db,
		table: table,
		embed: embed,
		dim:   dim,
	}
	if err := s.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库连接
func (s *PGVectorStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PGVectorStore) ensureSchema(ctx context.Context) error {
	// 启用 pgvector 扩展
	if _, err := s.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return fmt.Errorf("pgvector: failed to create extension: %w", err)
	}
	// 创建表
	createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		memory_id TEXT PRIMARY KEY,
		content TEXT,
		memory_type TEXT,
		memory_target TEXT,
		when_to_use TEXT,
		author TEXT,
		time_created TIMESTAMPTZ,
		time_modified TIMESTAMPTZ,
		message_time TIMESTAMPTZ,
		ref_memory_id TEXT,
		vector vector(%d)
	)`, s.table, s.dim)
	if _, err := s.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("pgvector: failed to create table: %w", err)
	}
	// 尝试创建 HNSW 索引（pgvector 0.5+）
	idxSQL := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING hnsw (vector vector_cosine_ops)`, s.table, s.table)
	if _, err := s.db.ExecContext(ctx, idxSQL); err != nil {
		// HNSW 可能不可用，回退到 IVFFlat
		idxSQL = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING ivfflat (vector vector_cosine_ops)`, s.table, s.table)
		_, _ = s.db.ExecContext(ctx, idxSQL) // 忽略错误，索引是可选优化
	}
	return nil
}

// Insert 插入记忆节点
func (s *PGVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s.embed == nil {
		return ErrEmbeddingRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt := fmt.Sprintf(`INSERT INTO %s (memory_id, content, memory_type, memory_target, when_to_use, author, time_created, time_modified, message_time, ref_memory_id, vector)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (memory_id) DO UPDATE SET
			content = EXCLUDED.content,
			memory_type = EXCLUDED.memory_type,
			memory_target = EXCLUDED.memory_target,
			when_to_use = EXCLUDED.when_to_use,
			author = EXCLUDED.author,
			time_created = EXCLUDED.time_created,
			time_modified = EXCLUDED.time_modified,
			message_time = EXCLUDED.message_time,
			ref_memory_id = EXCLUDED.ref_memory_id,
			vector = EXCLUDED.vector`, s.table)

	for _, node := range nodes {
		if node == nil {
			continue
		}
		if node.MemoryID == "" {
			node.MemoryID = GenerateMemoryID(node.Content)
		}
		if len(node.Vector) == 0 {
			v, err := s.embed.Embed(ctx, node.Content)
			if err != nil {
				return err
			}
			node.Vector = v
		}
		_, err := tx.ExecContext(ctx, stmt,
			node.MemoryID,
			node.Content,
			node.MemoryType,
			node.MemoryTarget,
			node.WhenToUse,
			node.Author,
			nullTime(node.TimeCreated),
			nullTime(node.TimeModified),
			nullTime(node.MessageTime),
			node.RefMemoryID,
			pgvector.NewVector(node.Vector),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Search 语义检索（使用 cosine distance）
func (s *PGVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if s.embed == nil {
		return nil, ErrEmbeddingRequired
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	limit := opts.TopK
	if limit <= 0 {
		limit = 10
	}

	whereClause, args := s.buildWhere(opts, 2)
	sqlText := fmt.Sprintf(`SELECT memory_id, content, memory_type, memory_target, when_to_use, author, time_created, time_modified, message_time, ref_memory_id, vector,
		1 - (vector <=> $1) AS score
	FROM %s
	%s
	ORDER BY vector <=> $1
	LIMIT $%d`, s.table, whereClause, len(args)+2)
	args = append([]any{pgvector.NewVector(qv)}, args...)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*MemoryNode
	for rows.Next() {
		n, err := s.scanNode(rows)
		if err != nil {
			return nil, err
		}
		if n.Score >= opts.MinScore {
			nodes = append(nodes, n)
		}
	}
	return nodes, rows.Err()
}

// Get 按 memoryID 读取
func (s *PGVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	sqlText := fmt.Sprintf(`SELECT memory_id, content, memory_type, memory_target, when_to_use, author, time_created, time_modified, message_time, ref_memory_id, vector, 0 AS score FROM %s WHERE memory_id = $1`, s.table)
	n, err := s.scanNode(s.db.QueryRowContext(ctx, sqlText, memoryID))
	if err == sql.ErrNoRows {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return n, nil
}

// Update 覆盖更新
func (s *PGVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	if node == nil || node.MemoryID == "" {
		return ErrInvalidMemoryNode
	}
	if len(node.Vector) == 0 {
		v, err := s.embed.Embed(ctx, node.Content)
		if err != nil {
			return err
		}
		node.Vector = v
	}
	sqlText := fmt.Sprintf(`UPDATE %s SET
		content = $1,
		memory_type = $2,
		memory_target = $3,
		when_to_use = $4,
		author = $5,
		time_created = $6,
		time_modified = $7,
		message_time = $8,
		ref_memory_id = $9,
		vector = $10
	WHERE memory_id = $11`, s.table)
	_, err := s.db.ExecContext(ctx, sqlText,
		node.Content,
		node.MemoryType,
		node.MemoryTarget,
		node.WhenToUse,
		node.Author,
		nullTime(node.TimeCreated),
		nullTime(node.TimeModified),
		nullTime(node.MessageTime),
		node.RefMemoryID,
		pgvector.NewVector(node.Vector),
		node.MemoryID,
	)
	return err
}

// Delete 按 memoryID 删除
func (s *PGVectorStore) Delete(ctx context.Context, memoryID string) error {
	sqlText := fmt.Sprintf(`DELETE FROM %s WHERE memory_id = $1`, s.table)
	_, err := s.db.ExecContext(ctx, sqlText, memoryID)
	return err
}

// DeleteAll 清空表
func (s *PGVectorStore) DeleteAll(ctx context.Context) error {
	sqlText := fmt.Sprintf(`DELETE FROM %s`, s.table)
	_, err := s.db.ExecContext(ctx, sqlText)
	return err
}

func (s *PGVectorStore) buildWhere(opts RetrieveOptions, argOffset int) (string, []any) {
	var conds []string
	var args []any
	if len(opts.MemoryTypes) > 0 {
		placeholders := make([]string, len(opts.MemoryTypes))
		for i, t := range opts.MemoryTypes {
			placeholders[i] = fmt.Sprintf("$%d", argOffset)
			args = append(args, string(t))
			argOffset++
		}
		conds = append(conds, fmt.Sprintf("memory_type IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(opts.MemoryTargets) > 0 {
		placeholders := make([]string, len(opts.MemoryTargets))
		for i, t := range opts.MemoryTargets {
			placeholders[i] = fmt.Sprintf("$%d", argOffset)
			args = append(args, t)
			argOffset++
		}
		conds = append(conds, fmt.Sprintf("memory_target IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

func (s *PGVectorStore) scanNode(scanner interface {
	Scan(dest ...any) error
}) (*MemoryNode, error) {
	var n MemoryNode
	var vec pgvector.Vector
	var score float64
	var tc, tm, mt sql.NullTime
	err := scanner.Scan(
		&n.MemoryID,
		&n.Content,
		&n.MemoryType,
		&n.MemoryTarget,
		&n.WhenToUse,
		&n.Author,
		&tc,
		&tm,
		&mt,
		&n.RefMemoryID,
		&vec,
		&score,
	)
	if err != nil {
		return nil, err
	}
	n.TimeCreated = tc.Time
	n.TimeModified = tm.Time
	if mt.Valid {
		n.MessageTime = mt.Time
	}
	n.Vector = vec.Slice()
	n.Score = score
	return &n, nil
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

var _ VectorStore = (*PGVectorStore)(nil)
