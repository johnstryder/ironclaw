package main

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"

	"ironclaw/internal/domain"

	_ "modernc.org/sqlite"
)

// fakeEmbedder returns a deterministic embedding based on text content.
// Uses simple character-sum hashing to produce a 3-dim vector.
type fakeEmbedder struct{}

func (f *fakeEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	// Create a deterministic vector from the text
	var sum float64
	for _, c := range text {
		sum += float64(c)
	}
	return []float64{sum * 0.001, (sum * 0.002) - 1.0, 1.0 - (sum * 0.0005)}, nil
}

var _ domain.Embedder = (*fakeEmbedder)(nil)

// =============================================================================
// Default factory tests
// =============================================================================

func TestDefaultEmbedderFactory_ShouldReturnOllamaEmbedder(t *testing.T) {
	e := embedderFactory()
	if e == nil {
		t.Fatal("expected non-nil embedder from default factory")
	}
}

func TestDefaultDBFactory_ShouldReturnOpenDB(t *testing.T) {
	db, err := dbFactory()
	if err != nil {
		t.Fatalf("expected no error from default db factory, got: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("expected db to be pingable, got: %v", err)
	}
}

// =============================================================================
// run tests
// =============================================================================

func TestRun_ShouldPrintStoredMemories(t *testing.T) {
	old := embedderFactory
	embedderFactory = func() domain.Embedder { return &fakeEmbedder{} }
	defer func() { embedderFactory = old }()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	output := stdout.String()
	if !strings.Contains(output, "Storing memories") {
		t.Errorf("expected 'Storing memories' header, got: %s", output)
	}
	if !strings.Contains(output, "Meeting is on Tuesday") {
		t.Errorf("expected stored meeting memory in output, got: %s", output)
	}
	if !strings.Contains(output, "3-dim") {
		t.Errorf("expected '3-dim' in output, got: %s", output)
	}
}

func TestRun_ShouldPrintSearchResults(t *testing.T) {
	old := embedderFactory
	embedderFactory = func() domain.Embedder { return &fakeEmbedder{} }
	defer func() { embedderFactory = old }()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	output := stdout.String()
	if !strings.Contains(output, "Semantic Search") {
		t.Errorf("expected 'Semantic Search' header, got: %s", output)
	}
	if !strings.Contains(output, "score=") {
		t.Errorf("expected score in output, got: %s", output)
	}
	if !strings.Contains(output, "Keyword Search") {
		t.Errorf("expected 'Keyword Search' header, got: %s", output)
	}
	if !strings.Contains(output, "Hybrid Search") {
		t.Errorf("expected 'Hybrid Search' header, got: %s", output)
	}
	if !strings.Contains(output, "Manual Test Complete") {
		t.Errorf("expected completion message, got: %s", output)
	}
}

func TestRun_ShouldExitWhenDBFails(t *testing.T) {
	oldDB := dbFactory
	oldExit := osExit
	var exitCode int
	osExit = func(code int) { exitCode = code }
	dbFactory = func() (*sql.DB, error) {
		db, _ := sql.Open("sqlite", ":memory:")
		db.Close() // Force it to fail on use
		return db, nil
	}
	defer func() {
		dbFactory = oldDB
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestRun_ShouldExitWhenEmbedFails(t *testing.T) {
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	osExit = func(code int) { exitCode = code }
	embedderFactory = func() domain.Embedder { return &failingEmbedder{} }
	defer func() {
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "embed") {
		t.Errorf("expected embed error in stderr, got: %s", stderr.String())
	}
}

type failingEmbedder struct{}

func (f *failingEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	return nil, context.DeadlineExceeded
}

func TestRun_ShouldExitWhenDBOpenFails(t *testing.T) {
	oldDB := dbFactory
	oldExit := osExit
	var exitCode int
	osExit = func(code int) { exitCode = code }
	dbFactory = func() (*sql.DB, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() {
		dbFactory = oldDB
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Cannot open database") {
		t.Errorf("expected db open error in stderr, got: %s", stderr.String())
	}
}

func TestMain_ShouldCallRun(t *testing.T) {
	// Just verify main() doesn't panic when factories are mocked
	oldEmbed := embedderFactory
	embedderFactory = func() domain.Embedder { return &fakeEmbedder{} }
	defer func() { embedderFactory = oldEmbed }()

	// Redirect stdout/stderr
	oldStdout := osExit
	osExit = func(code int) {}
	defer func() { osExit = oldStdout }()

	// main() writes to os.Stdout/os.Stderr directly, just make sure it doesn't panic
	main()
}

// =============================================================================
// Store error path
// =============================================================================

func TestRun_ShouldExitWhenStoreFails(t *testing.T) {
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	callCount := 0
	osExit = func(code int) { exitCode = code }

	// Embedder succeeds first call, returns a vector, then the DB is closed
	// between embed and store by the dbClosingEmbedder
	var sharedDB *sql.DB
	embedderFactory = func() domain.Embedder {
		return &dbClosingEmbedder{db: &sharedDB, count: &callCount}
	}

	oldDB := dbFactory
	dbFactory = func() (*sql.DB, error) {
		db, err := sql.Open("sqlite", ":memory:")
		sharedDB = db
		return db, err
	}
	defer func() {
		dbFactory = oldDB
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

// dbClosingEmbedder closes the DB after the first embed call so that Store fails.
type dbClosingEmbedder struct {
	db    **sql.DB
	count *int
}

func (d *dbClosingEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	*d.count++
	vec := []float64{0.1, 0.2, 0.3}
	// Close DB after returning the embedding so Store will fail
	if *d.db != nil {
		(*d.db).Close()
	}
	return vec, nil
}

// =============================================================================
// Search error path
// =============================================================================

func TestRun_ShouldExitWhenSemanticSearchEmbedFails(t *testing.T) {
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	callCount := 0
	osExit = func(code int) { exitCode = code }

	// Succeed for the 5 store embeds, fail on the first semantic query embed (call 6)
	embedderFactory = func() domain.Embedder {
		return &countingEmbedder{failAfter: 5, count: &callCount}
	}
	defer func() {
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

type countingEmbedder struct {
	failAfter int
	count     *int
}

func (c *countingEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	*c.count++
	if *c.count > c.failAfter {
		return nil, context.DeadlineExceeded
	}
	return []float64{0.1, 0.2, 0.3}, nil
}

func TestRun_ShouldExitWhenKeywordSearchFails(t *testing.T) {
	oldDB := dbFactory
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	callCount := 0
	osExit = func(code int) { exitCode = code }

	// Drop the FTS table on the 9th embed call (last semantic query embed).
	// Semantic search (vector-only) will still succeed, but keyword search
	// will fail because the memories_fts table is gone.
	var sharedDB *sql.DB
	embedderFactory = func() domain.Embedder {
		return &ftsBreakingEmbedder{
			breakOn: 9,
			count:   &callCount,
			db:      &sharedDB,
		}
	}
	dbFactory = func() (*sql.DB, error) {
		db, err := sql.Open("sqlite", ":memory:")
		sharedDB = db
		return db, err
	}
	defer func() {
		dbFactory = oldDB
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "keyword search") {
		t.Errorf("expected 'keyword search' error in stderr, got: %s", stderr.String())
	}
}

// ftsBreakingEmbedder drops the FTS table on a specific call so keyword search fails.
type ftsBreakingEmbedder struct {
	breakOn int
	count   *int
	db      **sql.DB
}

func (d *ftsBreakingEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	*d.count++
	if *d.count == d.breakOn && d.db != nil && *d.db != nil {
		(*d.db).Exec("DROP TABLE memories_fts")
	}
	return []float64{0.1, 0.2, 0.3}, nil
}

func TestRun_ShouldExitWhenHybridSearchEmbedFails(t *testing.T) {
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	callCount := 0
	osExit = func(code int) { exitCode = code }

	// Succeed for 5 stores + 4 semantic queries = 9 embeds, then fail on the
	// first hybrid query embed (call 10).
	embedderFactory = func() domain.Embedder {
		return &countingEmbedder{failAfter: 9, count: &callCount}
	}
	defer func() {
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "embed hybrid query") {
		t.Errorf("expected 'embed hybrid query' error in stderr, got: %s", stderr.String())
	}
}

func TestRun_ShouldExitWhenHybridSearchFails(t *testing.T) {
	oldDB := dbFactory
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	callCount := 0
	osExit = func(code int) { exitCode = code }

	// Close the DB after 10th embed call (5 stores + 4 semantic + 1 hybrid embed)
	// so that HybridSearch fails when it tries to query.
	var sharedDB *sql.DB
	embedderFactory = func() domain.Embedder {
		return &dbBreakingEmbedder{
			breakAfter: 10,
			count:      &callCount,
			db:         &sharedDB,
		}
	}
	dbFactory = func() (*sql.DB, error) {
		db, err := sql.Open("sqlite", ":memory:")
		sharedDB = db
		return db, err
	}
	defer func() {
		dbFactory = oldDB
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "hybrid search") {
		t.Errorf("expected 'hybrid search' error in stderr, got: %s", stderr.String())
	}
}

func TestRun_ShouldExitWhenSearchFails(t *testing.T) {
	oldDB := dbFactory
	oldEmbed := embedderFactory
	oldExit := osExit
	var exitCode int
	callCount := 0
	osExit = func(code int) { exitCode = code }

	// Close the DB from within the embedder on the 6th call (first query embed)
	// so that the embed succeeds but store.Search fails
	var sharedDB2 *sql.DB
	embedderFactory = func() domain.Embedder {
		return &dbBreakingEmbedder{
			breakAfter: 5,
			count:      &callCount,
			db:         &sharedDB2,
		}
	}
	dbFactory = func() (*sql.DB, error) {
		db, err := sql.Open("sqlite", ":memory:")
		sharedDB2 = db
		return db, err
	}
	defer func() {
		dbFactory = oldDB
		embedderFactory = oldEmbed
		osExit = oldExit
	}()

	var stdout, stderr bytes.Buffer
	run(&stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

// dbBreakingEmbedder closes the DB after breakAfter calls so Search fails.
type dbBreakingEmbedder struct {
	breakAfter int
	count      *int
	db         **sql.DB
}

func (d *dbBreakingEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	*d.count++
	// After storing all memories, close DB so Search will fail
	if *d.count == d.breakAfter+1 && d.db != nil && *d.db != nil {
		(*d.db).Close()
	}
	return []float64{0.1, 0.2, 0.3}, nil
}
