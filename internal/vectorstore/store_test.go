package vectorstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"testing"
	"time"

	"ironclaw/internal/domain"

	_ "modernc.org/sqlite"
)

// =============================================================================
// Test helpers
// =============================================================================

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// =============================================================================
// CosineSimilarity tests
// =============================================================================

func TestCosineSimilarity_ShouldReturnOneForIdenticalVectors(t *testing.T) {
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{1.0, 2.0, 3.0}
	got := CosineSimilarity(a, b)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("expected 1.0 for identical vectors, got %f", got)
	}
}

func TestCosineSimilarity_ShouldReturnZeroForOrthogonalVectors(t *testing.T) {
	a := []float64{1.0, 0.0}
	b := []float64{0.0, 1.0}
	got := CosineSimilarity(a, b)
	if math.Abs(got) > 1e-9 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", got)
	}
}

func TestCosineSimilarity_ShouldReturnNegativeOneForOppositeVectors(t *testing.T) {
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{-1.0, -2.0, -3.0}
	got := CosineSimilarity(a, b)
	if math.Abs(got-(-1.0)) > 1e-9 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", got)
	}
}

func TestCosineSimilarity_ShouldReturnZeroForEmptyVectors(t *testing.T) {
	got := CosineSimilarity([]float64{}, []float64{})
	if got != 0 {
		t.Errorf("expected 0.0 for empty vectors, got %f", got)
	}
}

func TestCosineSimilarity_ShouldReturnZeroForMismatchedLengths(t *testing.T) {
	a := []float64{1.0, 2.0}
	b := []float64{1.0, 2.0, 3.0}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("expected 0.0 for mismatched lengths, got %f", got)
	}
}

func TestCosineSimilarity_ShouldReturnZeroForZeroVector(t *testing.T) {
	a := []float64{0.0, 0.0, 0.0}
	b := []float64{1.0, 2.0, 3.0}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("expected 0.0 when one vector is zero, got %f", got)
	}
}

func TestCosineSimilarity_ShouldHandleHighDimensionVectors(t *testing.T) {
	// nomic-embed-text produces 768-dimensional vectors
	a := make([]float64, 768)
	b := make([]float64, 768)
	for i := range a {
		a[i] = float64(i)
		b[i] = float64(i)
	}
	got := CosineSimilarity(a, b)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("expected 1.0 for identical 768-dim vectors, got %f", got)
	}
}

// =============================================================================
// EncodeEmbedding / DecodeEmbedding tests
// =============================================================================

func TestEncodeEmbedding_ShouldRoundTrip(t *testing.T) {
	original := []float64{1.23, -4.56, 0.0, 7.89, math.Pi}
	encoded := EncodeEmbedding(original)
	decoded := DecodeEmbedding(encoded)

	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: want %d, got %d", len(original), len(decoded))
	}
	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("element %d: want %f, got %f", i, original[i], decoded[i])
		}
	}
}

func TestEncodeEmbedding_ShouldHandleEmptyVector(t *testing.T) {
	encoded := EncodeEmbedding([]float64{})
	if len(encoded) != 0 {
		t.Errorf("expected empty byte slice, got %d bytes", len(encoded))
	}
}

func TestDecodeEmbedding_ShouldHandleEmptyBlob(t *testing.T) {
	decoded := DecodeEmbedding([]byte{})
	if len(decoded) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(decoded))
	}
}

func TestEncodeEmbedding_ShouldProduceEightBytesPerFloat(t *testing.T) {
	vec := []float64{1.0, 2.0, 3.0}
	encoded := EncodeEmbedding(vec)
	if len(encoded) != 24 {
		t.Errorf("expected 24 bytes (3 * 8), got %d", len(encoded))
	}
}

func TestEncodeEmbedding_ShouldPreserveSpecialFloats(t *testing.T) {
	original := []float64{math.Inf(1), math.Inf(-1), math.SmallestNonzeroFloat64}
	decoded := DecodeEmbedding(EncodeEmbedding(original))
	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("element %d: want %v, got %v", i, original[i], decoded[i])
		}
	}
}

// =============================================================================
// NewSQLiteVectorStore tests
// =============================================================================

func TestNewSQLiteVectorStore_ShouldCreateMemoriesTable(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLiteVectorStore(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Verify the memories table exists
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='memories'").Scan(&name)
	if err != nil {
		t.Fatalf("memories table should exist: %v", err)
	}
	if name != "memories" {
		t.Errorf("expected table name 'memories', got %q", name)
	}
}

func TestNewSQLiteVectorStore_ShouldReturnErrorForNilDB(t *testing.T) {
	_, err := NewSQLiteVectorStore(nil)
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestNewSQLiteVectorStore_ShouldBeIdempotent(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLiteVectorStore(db)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call should not fail (CREATE TABLE IF NOT EXISTS)
	_, err = NewSQLiteVectorStore(db)
	if err != nil {
		t.Fatalf("second call should be idempotent: %v", err)
	}
}

// =============================================================================
// Store tests
// =============================================================================

func TestSQLiteVectorStore_Store_ShouldSaveMemory(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLiteVectorStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	embedding := []float64{0.1, 0.2, 0.3}
	err = store.Store(ctx, "Meeting is on Tuesday", embedding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify row was inserted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestSQLiteVectorStore_Store_ShouldRejectEmptyContent(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	err := store.Store(context.Background(), "", []float64{0.1, 0.2})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestSQLiteVectorStore_Store_ShouldRejectEmptyEmbedding(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	err := store.Store(context.Background(), "some content", []float64{})
	if err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestSQLiteVectorStore_Store_ShouldRejectNilEmbedding(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	err := store.Store(context.Background(), "some content", nil)
	if err == nil {
		t.Fatal("expected error for nil embedding")
	}
}

func TestSQLiteVectorStore_Store_ShouldPreserveEmbeddingPrecision(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	original := []float64{0.123456789012345, -9.87654321098765, math.Pi}
	err := store.Store(ctx, "precision test", original)
	if err != nil {
		t.Fatal(err)
	}

	// Read back the raw blob and decode
	var blob []byte
	db.QueryRow("SELECT embedding FROM memories WHERE content='precision test'").Scan(&blob)
	decoded := DecodeEmbedding(blob)

	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("element %d: want %.15f, got %.15f", i, original[i], decoded[i])
		}
	}
}

func TestSQLiteVectorStore_Store_ShouldSaveMultipleMemories(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	memories := []struct {
		content   string
		embedding []float64
	}{
		{"Meeting is on Tuesday", []float64{0.1, 0.2, 0.3}},
		{"Project deadline is Friday", []float64{0.4, 0.5, 0.6}},
		{"Lunch at noon", []float64{0.7, 0.8, 0.9}},
	}

	for _, m := range memories {
		if err := store.Store(ctx, m.content, m.embedding); err != nil {
			t.Fatalf("store %q: %v", m.content, err)
		}
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}
}

// =============================================================================
// Search tests
// =============================================================================

func TestSQLiteVectorStore_Search_ShouldReturnTopKResults(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// Store 3 memories with distinct embeddings
	store.Store(ctx, "Memory A", []float64{1.0, 0.0, 0.0})
	store.Store(ctx, "Memory B", []float64{0.0, 1.0, 0.0})
	store.Store(ctx, "Memory C", []float64{0.0, 0.0, 1.0})

	// Query with embedding closest to A
	results, err := store.Search(ctx, []float64{0.9, 0.1, 0.0}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestSQLiteVectorStore_Search_ShouldReturnMostSimilarFirst(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "cats are fluffy", []float64{0.9, 0.1, 0.0})
	store.Store(ctx, "dogs are loyal", []float64{0.1, 0.9, 0.0})
	store.Store(ctx, "fish swim fast", []float64{0.0, 0.1, 0.9})

	// Query embedding closest to "cats" vector
	results, err := store.Search(ctx, []float64{0.85, 0.15, 0.0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Content != "cats are fluffy" {
		t.Errorf("expected first result to be 'cats are fluffy', got %q", results[0].Content)
	}
	// Scores should be descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted descending: score[%d]=%f > score[%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestSQLiteVectorStore_Search_ShouldReturnEmptyWhenNoMemories(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	results, err := store.Search(context.Background(), []float64{1.0, 0.0}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSQLiteVectorStore_Search_ShouldRejectEmptyEmbedding(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.Search(context.Background(), []float64{}, 5)
	if err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestSQLiteVectorStore_Search_ShouldRejectNilEmbedding(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.Search(context.Background(), nil, 5)
	if err == nil {
		t.Fatal("expected error for nil embedding")
	}
}

func TestSQLiteVectorStore_Search_ShouldRejectZeroTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.Search(context.Background(), []float64{1.0}, 0)
	if err == nil {
		t.Fatal("expected error for zero topK")
	}
}

func TestSQLiteVectorStore_Search_ShouldRejectNegativeTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.Search(context.Background(), []float64{1.0}, -1)
	if err == nil {
		t.Fatal("expected error for negative topK")
	}
}

func TestSQLiteVectorStore_Search_ShouldHandleFewResults(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "only memory", []float64{1.0, 0.0})

	// Ask for 10 results when only 1 exists
	results, err := store.Search(ctx, []float64{1.0, 0.0}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (capped), got %d", len(results))
	}
}

func TestSQLiteVectorStore_Search_ShouldReturnScoresBetweenNegOneAndOne(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "positive", []float64{1.0, 0.0})
	store.Store(ctx, "negative", []float64{-1.0, 0.0})

	results, err := store.Search(ctx, []float64{1.0, 0.0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Score < -1.0 || r.Score > 1.0 {
			t.Errorf("score out of [-1,1] range: %f for %q", r.Score, r.Content)
		}
	}
}

func TestSQLiteVectorStore_Search_ShouldPopulateAllFields(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "test content", []float64{1.0, 0.0, 0.0})

	results, err := store.Search(ctx, []float64{1.0, 0.0, 0.0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ID <= 0 {
		t.Errorf("expected positive ID, got %d", r.ID)
	}
	if r.Content != "test content" {
		t.Errorf("expected content 'test content', got %q", r.Content)
	}
	if math.Abs(r.Score-1.0) > 1e-9 {
		t.Errorf("expected score 1.0 for identical vectors, got %f", r.Score)
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestSQLiteVectorStore_Search_ShouldReturnSemanticMemoryType(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "typing check", []float64{1.0})
	results, err := store.Search(ctx, []float64{1.0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Verify the result is a domain.SemanticMemory
	var _ []domain.SemanticMemory = results
}

// =============================================================================
// Store + Search roundtrip (E2E within unit)
// =============================================================================

func TestSQLiteVectorStore_E2E_StoreAndSearchShouldFindRelevantMemory(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// Simulate embeddings for different content
	meetingVec := []float64{0.8, 0.6, 0.0}
	projectVec := []float64{0.1, 0.9, 0.4}
	weatherVec := []float64{0.0, 0.1, 0.9}

	store.Store(ctx, "Meeting is on Tuesday at 10am", meetingVec)
	store.Store(ctx, "Project deadline is next Friday", projectVec)
	store.Store(ctx, "Weather will be sunny tomorrow", weatherVec)

	// Query: "What did we decide about the meeting?"
	// Simulated embedding close to meetingVec
	queryVec := []float64{0.75, 0.55, 0.05}
	results, err := store.Search(ctx, queryVec, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "Meeting is on Tuesday at 10am" {
		t.Errorf("expected meeting memory, got %q", results[0].Content)
	}
}

func TestSQLiteVectorStore_E2E_StoreAndSearchShouldFindMultipleRelevant(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// Two meeting-related memories with similar embeddings
	store.Store(ctx, "Meeting is on Tuesday", []float64{0.8, 0.6, 0.0})
	store.Store(ctx, "Meeting agenda: review Q4", []float64{0.7, 0.7, 0.1})
	store.Store(ctx, "Lunch is at noon", []float64{0.0, 0.1, 0.9})

	results, err := store.Search(ctx, []float64{0.75, 0.65, 0.05}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Both meeting memories should be returned
	for _, r := range results {
		if r.Content == "Lunch is at noon" {
			t.Error("lunch memory should not be in top 2 for meeting query")
		}
	}
}

// =============================================================================
// Store with cancelled context
// =============================================================================

func TestSQLiteVectorStore_Store_ShouldRespectCancelledContext(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := store.Store(ctx, "should fail", []float64{1.0})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestSQLiteVectorStore_Search_ShouldRespectCancelledContext(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	// Store one memory first
	store.Store(context.Background(), "test", []float64{1.0})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := store.Search(ctx, []float64{1.0}, 1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// =============================================================================
// Store with db errors
// =============================================================================

func TestSQLiteVectorStore_Store_ShouldReturnErrorWhenDBClosed(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	db.Close() // Close the DB to force an error

	err := store.Store(context.Background(), "test", []float64{1.0})
	if err == nil {
		t.Fatal("expected error when db is closed")
	}
}

func TestSQLiteVectorStore_Search_ShouldReturnErrorWhenDBClosed(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	db.Close()

	_, err := store.Search(context.Background(), []float64{1.0}, 1)
	if err == nil {
		t.Fatal("expected error when db is closed")
	}
}

// =============================================================================
// Migrate error test
// =============================================================================

func TestNewSQLiteVectorStore_ShouldReturnErrorWhenMigrateFails(t *testing.T) {
	db := openTestDB(t)
	db.Close() // Close to force migrate to fail

	_, err := NewSQLiteVectorStore(db)
	if err == nil {
		t.Fatal("expected error when migrate fails on closed db")
	}
}

// =============================================================================
// Search scan error paths
// =============================================================================

func TestSQLiteVectorStore_Search_ShouldReturnErrorWhenScanFails(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	// Insert a row with a NULL created_at to cause Scan to fail when scanning into time.Time
	_, err := db.Exec("INSERT INTO memories (content, embedding, created_at) VALUES (?, ?, NULL)", "bad row", EncodeEmbedding([]float64{1.0}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Search(context.Background(), []float64{1.0}, 1)
	if err == nil {
		t.Fatal("expected error when scan fails due to NULL created_at")
	}
}

func TestSQLiteVectorStore_Search_ShouldReturnErrorFromRowsErr(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	// Store a valid memory first so iteration occurs
	store.Store(context.Background(), "test", []float64{1.0})

	// Inject a rows.Err() hook to simulate an iteration error
	store.rowsErr = func() error { return fmt.Errorf("injected rows error") }

	_, err := store.Search(context.Background(), []float64{1.0}, 1)
	if err == nil {
		t.Fatal("expected error from rows.Err() hook")
	}
	if err.Error() != "injected rows error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

// =============================================================================
// FTS5 migration tests
// =============================================================================

func TestNewSQLiteVectorStore_ShouldCreateFTSTable(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLiteVectorStore(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the memories_fts virtual table exists
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='memories_fts'").Scan(&name)
	if err != nil {
		t.Fatalf("memories_fts table should exist: %v", err)
	}
	if name != "memories_fts" {
		t.Errorf("expected table name 'memories_fts', got %q", name)
	}
}

func TestSQLiteVectorStore_Store_ShouldPopulateFTSTable(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	err := store.Store(ctx, "The quick brown fox jumps over the lazy dog", []float64{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// Verify FTS5 table has data by doing a keyword match
	var content string
	err = db.QueryRow("SELECT content FROM memories_fts WHERE memories_fts MATCH 'fox'").Scan(&content)
	if err != nil {
		t.Fatalf("FTS query should find 'fox': %v", err)
	}
	if content != "The quick brown fox jumps over the lazy dog" {
		t.Errorf("unexpected FTS content: %q", content)
	}
}

// =============================================================================
// KeywordSearch tests
// =============================================================================

func TestSQLiteVectorStore_KeywordSearch_ShouldFindExactKeyword(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "The quick brown fox jumps over the lazy dog", []float64{0.1, 0.2, 0.3})
	store.Store(ctx, "A cat sat on the mat", []float64{0.4, 0.5, 0.6})
	store.Store(ctx, "Fox hunting is a traditional sport", []float64{0.7, 0.8, 0.9})

	results, err := store.KeywordSearch(ctx, "fox", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'fox', got %d", len(results))
	}
	// Both results should contain "fox" (case-insensitive)
	for _, r := range results {
		if r.Content != "The quick brown fox jumps over the lazy dog" &&
			r.Content != "Fox hunting is a traditional sport" {
			t.Errorf("unexpected result: %q", r.Content)
		}
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldReturnEmptyForNoMatch(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Meeting is on Tuesday", []float64{0.1, 0.2, 0.3})

	results, err := store.KeywordSearch(ctx, "elephant", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldRespectTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Meeting alpha on Monday", []float64{0.1, 0.2, 0.3})
	store.Store(ctx, "Meeting beta on Tuesday", []float64{0.4, 0.5, 0.6})
	store.Store(ctx, "Meeting gamma on Wednesday", []float64{0.7, 0.8, 0.9})

	results, err := store.KeywordSearch(ctx, "meeting", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (topK=2), got %d", len(results))
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldRejectEmptyQuery(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.KeywordSearch(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldRejectZeroTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.KeywordSearch(context.Background(), "test", 0)
	if err == nil {
		t.Fatal("expected error for zero topK")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldRejectNegativeTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.KeywordSearch(context.Background(), "test", -1)
	if err == nil {
		t.Fatal("expected error for negative topK")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldPopulateAllFields(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Meeting on Tuesday at 10am", []float64{1.0, 0.0, 0.0})

	results, err := store.KeywordSearch(ctx, "meeting", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ID <= 0 {
		t.Errorf("expected positive ID, got %d", r.ID)
	}
	if r.Content != "Meeting on Tuesday at 10am" {
		t.Errorf("expected content 'Meeting on Tuesday at 10am', got %q", r.Content)
	}
	if r.Score <= 0 {
		t.Errorf("expected positive score, got %f", r.Score)
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldReturnEmptyWhenNoMemories(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	results, err := store.KeywordSearch(context.Background(), "anything", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldHandleMultiWordQuery(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "The quick brown fox", []float64{0.1, 0.2, 0.3})
	store.Store(ctx, "A slow brown bear", []float64{0.4, 0.5, 0.6})
	store.Store(ctx, "Unrelated content here", []float64{0.7, 0.8, 0.9})

	// "brown fox" should match the first entry
	results, err := store.KeywordSearch(ctx, "brown fox", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'brown fox'")
	}
	// The first result should be the one with both words
	found := false
	for _, r := range results {
		if r.Content == "The quick brown fox" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'The quick brown fox' in results")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldRespectCancelledContext(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	store.Store(context.Background(), "test data", []float64{1.0})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.KeywordSearch(ctx, "test", 1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldReturnErrorWhenDBClosed(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	db.Close()

	_, err := store.KeywordSearch(context.Background(), "test", 1)
	if err == nil {
		t.Fatal("expected error when db is closed")
	}
}

// =============================================================================
// HybridSearch tests
// =============================================================================

func TestSQLiteVectorStore_HybridSearch_ShouldReturnSemanticAndKeywordResults(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// "budget" appears only in content 3, but embedding of content 1 is closest to query vector.
	// HybridSearch should surface both: semantic match (content 1) and keyword match (content 3).
	store.Store(ctx, "Meeting is on Tuesday", []float64{0.9, 0.1, 0.0})
	store.Store(ctx, "Weather will be sunny", []float64{0.0, 0.1, 0.9})
	store.Store(ctx, "The budget review is Friday", []float64{0.1, 0.9, 0.0})

	// Query vector is closest to "Meeting" embedding; keyword is "budget"
	results, err := store.HybridSearch(ctx, "budget", []float64{0.85, 0.15, 0.0}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find both: keyword match ("budget review") and semantic match ("Meeting")
	foundBudget := false
	foundMeeting := false
	for _, r := range results {
		if r.Content == "The budget review is Friday" {
			foundBudget = true
		}
		if r.Content == "Meeting is on Tuesday" {
			foundMeeting = true
		}
	}
	if !foundBudget {
		t.Error("expected keyword match 'The budget review is Friday' in results")
	}
	if !foundMeeting {
		t.Error("expected semantic match 'Meeting is on Tuesday' in results")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldDeduplicateResults(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// This memory will match both keyword ("meeting") and vector (closest embedding).
	store.Store(ctx, "Meeting is on Tuesday", []float64{0.9, 0.1, 0.0})
	store.Store(ctx, "Lunch is at noon", []float64{0.0, 0.1, 0.9})

	results, err := store.HybridSearch(ctx, "meeting", []float64{0.85, 0.15, 0.0}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count occurrences of "Meeting is on Tuesday" — should appear only once (deduped)
	count := 0
	for _, r := range results {
		if r.Content == "Meeting is on Tuesday" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 'Meeting is on Tuesday' exactly once (deduped), got %d", count)
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRespectTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Alpha meeting", []float64{0.9, 0.1, 0.0})
	store.Store(ctx, "Beta meeting", []float64{0.8, 0.2, 0.0})
	store.Store(ctx, "Gamma meeting", []float64{0.7, 0.3, 0.0})
	store.Store(ctx, "Delta meeting", []float64{0.6, 0.4, 0.0})
	store.Store(ctx, "Epsilon lunch", []float64{0.0, 0.1, 0.9})

	results, err := store.HybridSearch(ctx, "meeting", []float64{0.85, 0.15, 0.0}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results (topK=3), got %d", len(results))
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRejectEmptyQuery(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.HybridSearch(context.Background(), "", []float64{1.0}, 5)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRejectEmptyEmbedding(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.HybridSearch(context.Background(), "test", []float64{}, 5)
	if err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRejectNilEmbedding(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.HybridSearch(context.Background(), "test", nil, 5)
	if err == nil {
		t.Fatal("expected error for nil embedding")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRejectZeroTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.HybridSearch(context.Background(), "test", []float64{1.0}, 0)
	if err == nil {
		t.Fatal("expected error for zero topK")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRejectNegativeTopK(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	_, err := store.HybridSearch(context.Background(), "test", []float64{1.0}, -1)
	if err == nil {
		t.Fatal("expected error for negative topK")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldReturnEmptyWhenNoMemories(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	results, err := store.HybridSearch(context.Background(), "anything", []float64{1.0, 0.0}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldBoostDualMatchResults(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// "meeting" keyword match + close vector = should rank highest
	store.Store(ctx, "Meeting is on Tuesday", []float64{0.9, 0.1, 0.0})
	// "meeting" keyword match but far vector
	store.Store(ctx, "Another meeting on Friday", []float64{0.0, 0.1, 0.9})
	// No keyword match but close vector
	store.Store(ctx, "Appointment on Wednesday", []float64{0.85, 0.15, 0.0})

	results, err := store.HybridSearch(ctx, "meeting", []float64{0.9, 0.1, 0.0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// "Meeting is on Tuesday" should be first because it matches both keyword and vector
	if results[0].Content != "Meeting is on Tuesday" {
		t.Errorf("expected first result to be 'Meeting is on Tuesday', got %q", results[0].Content)
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldReturnResultsSortedByScore(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Meeting on Tuesday", []float64{0.9, 0.1, 0.0})
	store.Store(ctx, "Weather is sunny", []float64{0.0, 0.9, 0.1})
	store.Store(ctx, "Project deadline Friday", []float64{0.5, 0.5, 0.0})

	results, err := store.HybridSearch(ctx, "meeting", []float64{0.85, 0.15, 0.0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Verify scores are in descending order
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted descending: score[%d]=%f > score[%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldPopulateAllFields(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Meeting on Tuesday", []float64{1.0, 0.0, 0.0})

	results, err := store.HybridSearch(ctx, "meeting", []float64{1.0, 0.0, 0.0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.ID <= 0 {
		t.Errorf("expected positive ID, got %d", r.ID)
	}
	if r.Content == "" {
		t.Error("expected non-empty content")
	}
	if r.Score <= 0 {
		t.Errorf("expected positive score, got %f", r.Score)
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldWorkWithOnlyKeywordMatches(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	// Store memories where the keyword match is far from the query vector
	store.Store(ctx, "Specialized budgeting technique", []float64{0.0, 0.0, 1.0})
	store.Store(ctx, "Unrelated stuff about weather", []float64{0.9, 0.1, 0.0})

	// Query vector is close to "weather" but keyword is "budgeting"
	results, err := store.HybridSearch(ctx, "budgeting", []float64{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Should find the keyword match even though it has low vector similarity
	found := false
	for _, r := range results {
		if r.Content == "Specialized budgeting technique" {
			found = true
		}
	}
	if !found {
		t.Error("expected keyword-only match 'Specialized budgeting technique' in results")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldWorkWithOnlySemanticMatches(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Appointment on Wednesday", []float64{0.9, 0.1, 0.0})
	store.Store(ctx, "Lunch at noon", []float64{0.0, 0.1, 0.9})

	// Keyword "xyznonexistent" won't match anything, but vector search should still return results
	results, err := store.HybridSearch(ctx, "xyznonexistent", []float64{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least semantic results when keyword has no matches")
	}
	if results[0].Content != "Appointment on Wednesday" {
		t.Errorf("expected first result to be semantic match, got %q", results[0].Content)
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldRespectCancelledContext(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	store.Store(context.Background(), "test data", []float64{1.0})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.HybridSearch(ctx, "test", []float64{1.0}, 1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldReturnErrorWhenDBClosed(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	db.Close()

	_, err := store.HybridSearch(context.Background(), "test", []float64{1.0}, 1)
	if err == nil {
		t.Fatal("expected error when db is closed")
	}
}

func TestSQLiteVectorStore_HybridSearch_ShouldSwallowKeywordErrorAndReturnSemanticResults(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	ctx := context.Background()

	store.Store(ctx, "Meeting on Tuesday", []float64{0.9, 0.1, 0.0})

	// An unbalanced double-quote is invalid FTS5 syntax, causing KeywordSearch to error.
	// HybridSearch should swallow that error and return semantic-only results.
	results, err := store.HybridSearch(ctx, "\"", []float64{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("expected no error (keyword error swallowed), got: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected semantic results even when keyword search errors")
	}
}

// =============================================================================
// KeywordSearch error paths
// =============================================================================

func TestSQLiteVectorStore_KeywordSearch_ShouldReturnErrorWhenScanFails(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	// Insert into memories with NULL created_at and manually insert into FTS5
	_, err := db.Exec("INSERT INTO memories (content, embedding, created_at) VALUES (?, ?, NULL)",
		"bad row", EncodeEmbedding([]float64{1.0}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO memories_fts(rowid, content) VALUES (1, ?)", "bad row")
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.KeywordSearch(context.Background(), "bad", 1)
	if err == nil {
		t.Fatal("expected error when scan fails due to NULL created_at")
	}
}

func TestSQLiteVectorStore_KeywordSearch_ShouldReturnErrorFromRowsErr(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)
	store.Store(context.Background(), "test content", []float64{1.0})

	// Inject a rows.Err() hook to simulate an iteration error
	store.rowsErr = func() error { return fmt.Errorf("injected keyword rows error") }

	_, err := store.KeywordSearch(context.Background(), "test", 1)
	if err == nil {
		t.Fatal("expected error from rows.Err() hook")
	}
	if err.Error() != "injected keyword rows error" {
		t.Errorf("expected injected error, got %v", err)
	}
}

// =============================================================================
// Store FTS insert error
// =============================================================================

func TestSQLiteVectorStore_Store_ShouldReturnErrorWhenFTSInsertFails(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	// Drop the FTS table so the FTS insert fails
	db.Exec("DROP TABLE memories_fts")

	err := store.Store(context.Background(), "test", []float64{1.0})
	if err == nil {
		t.Fatal("expected error when FTS insert fails")
	}
}

func TestSQLiteVectorStore_Store_ShouldReturnErrorWhenLastInsertIDFails(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	// Inject a failing LastInsertId hook
	store.lastInsertID = func(r sql.Result) (int64, error) {
		return 0, fmt.Errorf("injected last insert id error")
	}

	err := store.Store(context.Background(), "test content", []float64{1.0})
	if err == nil {
		t.Fatal("expected error when LastInsertId fails")
	}
	if err.Error() != "get last insert id: injected last insert id error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// mergeAndRank edge case tests
// =============================================================================

func TestMergeAndRank_ShouldReturnEmptyForBothEmptyInputs(t *testing.T) {
	result := mergeAndRank(nil, nil, 5)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestMergeAndRank_ShouldCapTopKToAvailableResults(t *testing.T) {
	semantic := []domain.SemanticMemory{
		{ID: 1, Content: "A", Score: 0.9},
	}
	keyword := []domain.SemanticMemory{
		{ID: 2, Content: "B", Score: 0.8},
	}
	// topK=10 but only 2 unique results
	result := mergeAndRank(semantic, keyword, 10)
	if len(result) != 2 {
		t.Errorf("expected 2 results (capped from topK=10), got %d", len(result))
	}
}

func TestMergeAndRank_ShouldTruncateToTopK(t *testing.T) {
	semantic := []domain.SemanticMemory{
		{ID: 1, Content: "A", Score: 0.9},
		{ID: 2, Content: "B", Score: 0.8},
		{ID: 3, Content: "C", Score: 0.7},
	}
	keyword := []domain.SemanticMemory{
		{ID: 4, Content: "D", Score: 0.6},
		{ID: 5, Content: "E", Score: 0.5},
	}
	// topK=3 but 5 unique results available
	result := mergeAndRank(semantic, keyword, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 results (topK=3), got %d", len(result))
	}
}

func TestMergeAndRank_ShouldBoostDuplicateIDs(t *testing.T) {
	semantic := []domain.SemanticMemory{
		{ID: 1, Content: "Shared", Score: 0.9},
		{ID: 2, Content: "SemanticOnly", Score: 0.8},
	}
	keyword := []domain.SemanticMemory{
		{ID: 1, Content: "Shared", Score: 0.7},
		{ID: 3, Content: "KeywordOnly", Score: 0.6},
	}
	result := mergeAndRank(semantic, keyword, 5)

	// ID=1 should have the highest combined RRF score since it appears in both
	if result[0].ID != 1 {
		t.Errorf("expected ID=1 (dual match) to be first, got ID=%d", result[0].ID)
	}
	// Its score should be the sum of both RRF contributions
	expectedScore := 1.0/float64(rrfK+1) + 1.0/float64(rrfK+1)
	if math.Abs(result[0].Score-expectedScore) > 1e-9 {
		t.Errorf("expected combined RRF score %f, got %f", expectedScore, result[0].Score)
	}
}

// =============================================================================
// CreatedAt timing
// =============================================================================

func TestSQLiteVectorStore_Store_ShouldSetCreatedAtToNow(t *testing.T) {
	db := openTestDB(t)
	store, _ := NewSQLiteVectorStore(db)

	before := time.Now().Add(-time.Second)
	store.Store(context.Background(), "timing test", []float64{1.0, 0.0})
	after := time.Now().Add(time.Second)

	results, _ := store.Search(context.Background(), []float64{1.0, 0.0}, 1)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	created := results[0].CreatedAt
	if created.Before(before) || created.After(after) {
		t.Errorf("CreatedAt %v not between %v and %v", created, before, after)
	}
}
