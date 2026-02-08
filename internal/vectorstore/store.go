package vectorstore

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"

	"ironclaw/internal/domain"
)

// rowsErrFunc is a function type for testing the rows.Err() error path.
type rowsErrFunc func() error

// lastInsertIDFunc wraps sql.Result.LastInsertId for testing error paths.
type lastInsertIDFunc func(sql.Result) (int64, error)

// SQLiteVectorStore stores memories and their embeddings in SQLite.
// Vector search is done via in-memory cosine similarity computation.
type SQLiteVectorStore struct {
	db           *sql.DB
	rowsErr      rowsErrFunc      // nil means use rows.Err(); for testing only
	lastInsertID lastInsertIDFunc // nil means use res.LastInsertId(); for testing only
}

// NewSQLiteVectorStore creates a new vector store and initializes the schema.
// Returns an error if the db is nil or if the migration fails.
func NewSQLiteVectorStore(db *sql.DB) (*SQLiteVectorStore, error) {
	if db == nil {
		return nil, fmt.Errorf("db must not be nil")
	}
	s := &SQLiteVectorStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("vectorstore migrate: %w", err)
	}
	return s, nil
}

// migrate creates the memories table and FTS5 virtual table if they don't exist.
func (s *SQLiteVectorStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(content)
	`)
	return err
}

// Store saves content with its embedding vector and indexes it for full-text search.
func (s *SQLiteVectorStore) Store(ctx context.Context, content string, embedding []float64) error {
	if content == "" {
		return fmt.Errorf("content must not be empty")
	}
	if len(embedding) == 0 {
		return fmt.Errorf("embedding must not be empty")
	}
	blob := EncodeEmbedding(embedding)
	res, err := s.db.ExecContext(ctx, "INSERT INTO memories (content, embedding) VALUES (?, ?)", content, blob)
	if err != nil {
		return err
	}
	getID := func(r sql.Result) (int64, error) { return r.LastInsertId() }
	if s.lastInsertID != nil {
		getID = s.lastInsertID
	}
	id, err := getID(res)
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO memories_fts(rowid, content) VALUES (?, ?)", id, content)
	return err
}

// Search finds the top K most similar memories to the query embedding.
// Results are sorted by cosine similarity in descending order.
func (s *SQLiteVectorStore) Search(ctx context.Context, embedding []float64, topK int) ([]domain.SemanticMemory, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding must not be empty")
	}
	if topK <= 0 {
		return nil, fmt.Errorf("topK must be positive")
	}

	rows, err := s.db.QueryContext(ctx, "SELECT id, content, embedding, created_at FROM memories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		id        int64
		content   string
		score     float64
		createdAt time.Time
	}

	var candidates []candidate
	for rows.Next() {
		var id int64
		var content string
		var blob []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &content, &blob, &createdAt); err != nil {
			return nil, err
		}
		stored := DecodeEmbedding(blob)
		score := CosineSimilarity(embedding, stored)
		candidates = append(candidates, candidate{id, content, score, createdAt})
	}
	rowsErr := rows.Err()
	if s.rowsErr != nil {
		rowsErr = s.rowsErr()
	}
	if rowsErr != nil {
		return nil, rowsErr
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Take top K (capped to available results)
	if topK > len(candidates) {
		topK = len(candidates)
	}

	result := make([]domain.SemanticMemory, topK)
	for i := 0; i < topK; i++ {
		result[i] = domain.SemanticMemory{
			ID:        candidates[i].id,
			Content:   candidates[i].content,
			Score:     candidates[i].score,
			CreatedAt: candidates[i].createdAt,
		}
	}

	return result, nil
}

// HybridSearch runs both a vector (semantic) search and an FTS5 (keyword) search,
// merges the results, deduplicates by memory ID, and re-ranks using Reciprocal Rank
// Fusion (RRF). Results that appear in both searches get a score boost.
// The constant k=60 is standard for RRF.
func (s *SQLiteVectorStore) HybridSearch(ctx context.Context, query string, embedding []float64, topK int) ([]domain.SemanticMemory, error) {
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding must not be empty")
	}
	if topK <= 0 {
		return nil, fmt.Errorf("topK must be positive")
	}

	// Run vector search
	semanticResults, err := s.Search(ctx, embedding, topK)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	// Run keyword search (FTS5 MATCH may fail for non-matching terms — that's OK)
	keywordResults, err := s.KeywordSearch(ctx, query, topK)
	if err != nil {
		// FTS5 query syntax errors are not fatal; treat as empty keyword results
		keywordResults = nil
	}

	return mergeAndRank(semanticResults, keywordResults, topK), nil
}

// rrfK is the Reciprocal Rank Fusion constant.
// A standard value of 60 balances between top and lower-ranked results.
const rrfK = 60

// mergeAndRank combines semantic and keyword results using Reciprocal Rank Fusion.
// Duplicate memories (same ID) receive scores from both lists.
func mergeAndRank(semantic, keyword []domain.SemanticMemory, topK int) []domain.SemanticMemory {
	type scored struct {
		mem   domain.SemanticMemory
		score float64
	}
	seen := make(map[int64]*scored)

	// Add semantic results with RRF scores
	for rank, m := range semantic {
		s := &scored{mem: m, score: 1.0 / float64(rrfK+rank+1)}
		seen[m.ID] = s
	}

	// Add keyword results — boost if already present
	for rank, m := range keyword {
		rrfScore := 1.0 / float64(rrfK+rank+1)
		if existing, ok := seen[m.ID]; ok {
			existing.score += rrfScore
		} else {
			seen[m.ID] = &scored{mem: m, score: rrfScore}
		}
	}

	// Collect and sort by combined score descending
	all := make([]scored, 0, len(seen))
	for _, s := range seen {
		all = append(all, *s)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].score > all[j].score
	})

	if topK > len(all) {
		topK = len(all)
	}
	result := make([]domain.SemanticMemory, topK)
	for i := 0; i < topK; i++ {
		result[i] = all[i].mem
		result[i].Score = all[i].score
	}
	return result
}

// KeywordSearch finds memories matching the query using FTS5 full-text search.
// Results are sorted by FTS5 relevance (best match first) and limited to topK.
// The Score field is set to the negated FTS5 rank (higher = better match).
func (s *SQLiteVectorStore) KeywordSearch(ctx context.Context, query string, topK int) ([]domain.SemanticMemory, error) {
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}
	if topK <= 0 {
		return nil, fmt.Errorf("topK must be positive")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.content, m.created_at, f.rank
		FROM memories_fts f
		JOIN memories m ON m.id = f.rowid
		WHERE memories_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`, query, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.SemanticMemory
	for rows.Next() {
		var id int64
		var content string
		var createdAt time.Time
		var rank float64
		if err := rows.Scan(&id, &content, &createdAt, &rank); err != nil {
			return nil, err
		}
		// FTS5 rank is negative (more negative = better). Negate it for a positive score.
		results = append(results, domain.SemanticMemory{
			ID:        id,
			Content:   content,
			Score:     -rank,
			CreatedAt: createdAt,
		})
	}
	rowsErr := rows.Err()
	if s.rowsErr != nil {
		rowsErr = s.rowsErr()
	}
	if rowsErr != nil {
		return nil, rowsErr
	}
	return results, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 for empty, zero, or mismatched-length vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// EncodeEmbedding converts a float64 slice to a byte slice for SQLite BLOB storage.
// Each float64 is stored as 8 bytes in little-endian format.
func EncodeEmbedding(vec []float64) []byte {
	buf := make([]byte, len(vec)*8)
	for i, v := range vec {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
	}
	return buf
}

// DecodeEmbedding converts a byte slice back to a float64 slice.
func DecodeEmbedding(data []byte) []float64 {
	n := len(data) / 8
	vec := make([]float64, n)
	for i := range vec {
		vec[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return vec
}
