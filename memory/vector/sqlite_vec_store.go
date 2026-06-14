package vector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

// SQLiteVecStore is a VectorStore backed by SQLite + the sqlite-vec extension.
// It provides persistent vector storage with zero CGO dependencies.
//
// The store uses two tables:
//   - vec_items (vec0 virtual table): id TEXT PRIMARY KEY, embedding float[dim]
//   - vec_metadata (regular table): full MemoryNode fields as columns
//
// Vectors are normalized before insertion so L2 distance can be converted to
// cosine similarity: cos_sim = 1 - L2² / 2.
type SQLiteVecStore struct {
	db    *sql.DB
	embed EmbeddingModel
	dim   int
	mu    sync.Mutex
}

// NewSQLiteVecStore creates a persistent SQLite vector store.
// dbPath is the SQLite database file path (use ":memory:" for in-memory).
// dim is the embedding dimension (must be > 0).
// embed provides text-to-vector conversion for Search queries and auto-embedding.
func NewSQLiteVecStore(dbPath string, dim int, embed EmbeddingModel) (*SQLiteVecStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if dim <= 0 {
		return nil, fmt.Errorf("vector: dimension must be positive, got %d", dim)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("vector: open sqlite: %w", err)
	}

	s := &SQLiteVecStore{db: db, embed: embed, dim: dim}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// NewSQLiteVecStoreFromDB creates a SQLiteVecStore from an existing *sql.DB connection.
// The DB must already have the sqlite-vec extension loaded (via the modernc.org/sqlite/vec import).
func NewSQLiteVecStoreFromDB(db *sql.DB, dim int, embed EmbeddingModel) (*SQLiteVecStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if dim <= 0 {
		return nil, fmt.Errorf("vector: dimension must be positive, got %d", dim)
	}
	if db == nil {
		return nil, fmt.Errorf("vector: nil db")
	}
	s := &SQLiteVecStore{db: db, embed: embed, dim: dim}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteVecStore) init() error {
	_, err := s.db.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_items USING vec0(
			id TEXT PRIMARY KEY,
			embedding float[%d]
		);
	`, s.dim))
	if err != nil {
		return fmt.Errorf("vector: create vec_items: %w", err)
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS vec_metadata (
			memory_id      TEXT PRIMARY KEY,
			memory_type    TEXT NOT NULL DEFAULT '',
			memory_target  TEXT NOT NULL DEFAULT '',
			when_to_use    TEXT NOT NULL DEFAULT '',
			content        TEXT NOT NULL DEFAULT '',
			message_time   TEXT,
			ref_memory_id  TEXT NOT NULL DEFAULT '',
			time_created   TEXT,
			time_modified  TEXT,
			author         TEXT NOT NULL DEFAULT '',
			metadata_json  TEXT NOT NULL DEFAULT '{}'
		);
	`)
	if err != nil {
		return fmt.Errorf("vector: create vec_metadata: %w", err)
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_vec_meta_type ON vec_metadata(memory_type);`)
	if err != nil {
		return fmt.Errorf("vector: create index: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteVecStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Insert stores memory nodes with their vectors.
// Nodes without a Vector will be auto-embedded using the EmbeddingModel.
func (s *SQLiteVecStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		if node.MemoryID == "" {
			node.MemoryID = GenerateMemoryID(node.EmbeddingContent())
		}
		if len(node.Vector) == 0 {
			vec, err := s.embed.Embed(ctx, node.EmbeddingContent())
			if err != nil {
				return fmt.Errorf("vector: embed %s: %w", node.MemoryID, err)
			}
			node.Vector = vec
		}
		if len(node.Vector) != s.dim {
			return fmt.Errorf("%w: expected %d, got %d", ErrVectorDimension, s.dim, len(node.Vector))
		}
		normalizeVector(node.Vector)

		vecJSON := vectorToJSON(node.Vector)
		metaJSON := metadataToJSON(node.Metadata)

		now := time.Now().UTC()
		if node.TimeCreated.IsZero() {
			node.TimeCreated = now
		}
		if node.TimeModified.IsZero() {
			node.TimeModified = now
		}

		_, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO vec_metadata
			(memory_id, memory_type, memory_target, when_to_use, content,
			 message_time, ref_memory_id, time_created, time_modified, author, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
		`,
			node.MemoryID, string(node.MemoryType), node.MemoryTarget, node.WhenToUse,
			node.Content, formatTime(node.MessageTime), node.RefMemoryID,
			formatTime(node.TimeCreated), formatTime(node.TimeModified), node.Author, metaJSON,
		)
		if err != nil {
			return fmt.Errorf("vector: insert metadata %s: %w", node.MemoryID, err)
		}

		_, err = s.db.ExecContext(ctx, `DELETE FROM vec_items WHERE id = ?;`, node.MemoryID)
		if err != nil {
			return fmt.Errorf("vector: delete old vec %s: %w", node.MemoryID, err)
		}

		_, err = s.db.ExecContext(ctx,
			`INSERT INTO vec_items(id, embedding) VALUES (?, vec_f32(?));`,
			node.MemoryID, vecJSON,
		)
		if err != nil {
			return fmt.Errorf("vector: insert vec %s: %w", node.MemoryID, err)
		}
	}
	return nil
}

// Search performs a similarity search against stored vectors.
func (s *SQLiteVecStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	queryVec, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("vector: embed query: %w", err)
	}
	if len(queryVec) != s.dim {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrVectorDimension, s.dim, len(queryVec))
	}
	normalizeVector(queryVec)

	queryJSON := vectorToJSON(queryVec)

	sql := `SELECT v.id, v.distance, m.memory_type, m.memory_target, m.when_to_use,
			m.content, m.message_time, m.ref_memory_id, m.time_created, m.time_modified,
			m.author, m.metadata_json
		FROM vec_items v
		JOIN vec_metadata m ON v.id = m.memory_id
		WHERE v.embedding MATCH vec_f32(?) AND v.k = ?`

	args := []any{queryJSON, opts.TopK * 3}

	if len(opts.MemoryTypes) > 0 {
		placeholders := make([]string, len(opts.MemoryTypes))
		for i, t := range opts.MemoryTypes {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		sql += " AND m.memory_type IN (" + strings.Join(placeholders, ",") + ")"
	}
	if len(opts.MemoryTargets) > 0 {
		placeholders := make([]string, len(opts.MemoryTargets))
		for i, t := range opts.MemoryTargets {
			placeholders[i] = "?"
			args = append(args, t)
		}
		sql += " AND m.memory_target IN (" + strings.Join(placeholders, ",") + ")"
	}

	sql += " ORDER BY v.distance LIMIT ?"
	args = append(args, opts.TopK)

	rows, err := s.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("vector: search query: %w", err)
	}
	defer rows.Close()

	var results []*MemoryNode
	for rows.Next() {
		var id string
		var distance float64
		var memType, memTarget, whenToUse, content string
		var msgTime, refID, timeCreated, timeModified, author, metaJSON string

		if err := rows.Scan(&id, &distance, &memType, &memTarget, &whenToUse,
			&content, &msgTime, &refID, &timeCreated, &timeModified, &author, &metaJSON); err != nil {
			return nil, fmt.Errorf("vector: scan row: %w", err)
		}

		score := 1.0 - distance*distance/2.0
		if score < 0 {
			score = 0
		}

		node := &MemoryNode{
			MemoryID:     id,
			MemoryType:   MemoryType(memType),
			MemoryTarget: memTarget,
			WhenToUse:    whenToUse,
			Content:      content,
			RefMemoryID:  refID,
			Author:       author,
			Score:        score,
			Metadata:     jsonToMetadata(metaJSON),
		}
		node.MessageTime = parseTime(msgTime)
		node.TimeCreated = parseTime(timeCreated)
		node.TimeModified = parseTime(timeModified)

		if opts.MinScore > 0 && score < opts.MinScore {
			continue
		}
		results = append(results, node)
	}

	return results, rows.Err()
}

// Get retrieves a single memory node by ID.
func (s *SQLiteVecStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var memType, memTarget, whenToUse, content string
	var msgTime, refID, timeCreated, timeModified, author, metaJSON string

	err := s.db.QueryRowContext(ctx, `
		SELECT memory_type, memory_target, when_to_use, content,
		       message_time, ref_memory_id, time_created, time_modified, author, metadata_json
		FROM vec_metadata WHERE memory_id = ?
	`, memoryID).Scan(&memType, &memTarget, &whenToUse, &content,
		&msgTime, &refID, &timeCreated, &timeModified, &author, &metaJSON)

	if err == sql.ErrNoRows {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("vector: get %s: %w", memoryID, err)
	}

	node := &MemoryNode{
		MemoryID:     memoryID,
		MemoryType:   MemoryType(memType),
		MemoryTarget: memTarget,
		WhenToUse:    whenToUse,
		Content:      content,
		RefMemoryID:  refID,
		Author:       author,
		Metadata:     jsonToMetadata(metaJSON),
	}
	node.MessageTime = parseTime(msgTime)
	node.TimeCreated = parseTime(timeCreated)
	node.TimeModified = parseTime(timeModified)
	return node, nil
}

// Update modifies an existing memory node.
func (s *SQLiteVecStore) Update(ctx context.Context, node *MemoryNode) error {
	if node == nil || node.MemoryID == "" {
		return ErrInvalidMemoryNode
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(node.Vector) > 0 {
		if len(node.Vector) != s.dim {
			return fmt.Errorf("%w: expected %d, got %d", ErrVectorDimension, s.dim, len(node.Vector))
		}
		normalizeVector(node.Vector)
		vecJSON := vectorToJSON(node.Vector)
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM vec_items WHERE id = ?;`, node.MemoryID)
		if err != nil {
			return fmt.Errorf("vector: update vec delete: %w", err)
		}
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO vec_items(id, embedding) VALUES (?, vec_f32(?));`,
			node.MemoryID, vecJSON)
		if err != nil {
			return fmt.Errorf("vector: update vec insert: %w", err)
		}
	}

	node.TimeModified = time.Now().UTC()
	metaJSON := metadataToJSON(node.Metadata)

	_, err := s.db.ExecContext(ctx, `
		UPDATE vec_metadata SET
			memory_type = ?, memory_target = ?, when_to_use = ?, content = ?,
			message_time = ?, ref_memory_id = ?, time_modified = ?, author = ?, metadata_json = ?
		WHERE memory_id = ?;
	`,
		string(node.MemoryType), node.MemoryTarget, node.WhenToUse, node.Content,
		formatTime(node.MessageTime), node.RefMemoryID, formatTime(node.TimeModified),
		node.Author, metaJSON, node.MemoryID,
	)
	if err != nil {
		return fmt.Errorf("vector: update metadata: %w", err)
	}
	return nil
}

// Delete removes a memory node and its vector.
func (s *SQLiteVecStore) Delete(ctx context.Context, memoryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM vec_items WHERE id = ?;`, memoryID)
	if err != nil {
		return fmt.Errorf("vector: delete vec: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM vec_metadata WHERE memory_id = ?;`, memoryID)
	if err != nil {
		return fmt.Errorf("vector: delete metadata: %w", err)
	}
	return nil
}

// DeleteAll removes all memory nodes and vectors.
func (s *SQLiteVecStore) DeleteAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM vec_items;`)
	if err != nil {
		return fmt.Errorf("vector: deleteall vec: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM vec_metadata;`)
	if err != nil {
		return fmt.Errorf("vector: deleteall metadata: %w", err)
	}
	return nil
}

// --- Helpers ---

func normalizeVector(v []float32) {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
}

func vectorToJSON(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%g", x))
	}
	b.WriteByte(']')
	return b.String()
}

func metadataToJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func jsonToMetadata(s string) map[string]any {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

var _ VectorStore = (*SQLiteVecStore)(nil)
