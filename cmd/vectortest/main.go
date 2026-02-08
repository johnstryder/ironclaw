// vectortest is a manual integration test for semantic vector search.
// It requires Ollama running with the nomic-embed-text model.
//
// Usage:
//
//	go run ./cmd/vectortest/
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	_ "modernc.org/sqlite"

	"ironclaw/internal/domain"
	"ironclaw/internal/embedding"
	"ironclaw/internal/vectorstore"
)

// embedderFactory creates the Embedder. Package-level for test injection.
var embedderFactory = func() domain.Embedder {
	return embedding.NewOllamaEmbedder("nomic-embed-text")
}

// dbFactory opens the database. Package-level for test injection.
var dbFactory = func() (*sql.DB, error) {
	return sql.Open("sqlite", ":memory:")
}

// osExit wraps os.Exit so tests can capture exit calls.
var osExit = os.Exit

func main() {
	run(os.Stdout, os.Stderr)
}

func run(stdout, stderr io.Writer) {
	fmt.Fprintln(stdout, "=== Semantic Vector Search Manual Test ===")
	fmt.Fprintln(stdout, "Requires: Ollama running with nomic-embed-text")
	fmt.Fprintln(stdout)

	db, err := dbFactory()
	if err != nil {
		fmt.Fprintf(stderr, "FAIL: Cannot open database: %v\n", err)
		osExit(1)
		return
	}
	defer db.Close()

	store, err := vectorstore.NewSQLiteVectorStore(db)
	if err != nil {
		fmt.Fprintf(stderr, "FAIL: Cannot create vector store: %v\n", err)
		osExit(1)
		return
	}

	embedder := embedderFactory()
	ctx := context.Background()

	// Store memories
	memories := []string{
		"Meeting is on Tuesday at 10am in the conference room",
		"Project deadline is next Friday",
		"Weather will be sunny tomorrow",
		"John's favorite color is blue",
		"The budget review happens every Monday",
	}

	fmt.Fprintln(stdout, "--- Storing memories ---")
	for _, m := range memories {
		vec, err := embedder.Embed(ctx, m)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL: embed %q: %v\n", m, err)
			osExit(1)
			return
		}
		if err := store.Store(ctx, m, vec); err != nil {
			fmt.Fprintf(stderr, "FAIL: store %q: %v\n", m, err)
			osExit(1)
			return
		}
		fmt.Fprintf(stdout, "  [OK] %q (%d-dim)\n", m, len(vec))
	}

	// Semantic search with different phrasing
	queries := []string{
		"What did we decide about the meeting?",
		"When is the project due?",
		"What color does John like?",
		"Is it going to rain?",
	}

	fmt.Fprintln(stdout, "\n--- Semantic Search ---")
	for _, q := range queries {
		fmt.Fprintf(stdout, "\nQuery: %q\n", q)
		qvec, err := embedder.Embed(ctx, q)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL: embed query: %v\n", err)
			osExit(1)
			return
		}
		results, err := store.Search(ctx, qvec, 2)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL: search: %v\n", err)
			osExit(1)
			return
		}
		for i, r := range results {
			fmt.Fprintf(stdout, "  #%d [score=%.4f] %s\n", i+1, r.Score, r.Content)
		}
	}

	// Keyword search using FTS5
	keywords := []string{
		"budget",
		"Tuesday",
		"sunny",
		"color blue",
		"xyznonexistent",
	}

	fmt.Fprintln(stdout, "\n--- Keyword Search (FTS5) ---")
	for _, kw := range keywords {
		fmt.Fprintf(stdout, "\nKeyword: %q\n", kw)
		results, err := store.KeywordSearch(ctx, kw, 3)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL: keyword search %q: %v\n", kw, err)
			osExit(1)
			return
		}
		if len(results) == 0 {
			fmt.Fprintln(stdout, "  (no matches)")
		}
		for i, r := range results {
			fmt.Fprintf(stdout, "  #%d [score=%.4f] %s\n", i+1, r.Score, r.Content)
		}
	}

	// Hybrid search: keyword + vector combined
	type hybridQuery struct {
		keyword string
		text    string // used to generate the embedding
	}
	hybridQueries := []hybridQuery{
		{keyword: "budget", text: "What about the budget?"},
		{keyword: "meeting", text: "When is the next meeting?"},
		{keyword: "Friday", text: "What happens on Friday?"},
		{keyword: "weather", text: "Tell me about the weather"},
	}

	fmt.Fprintln(stdout, "\n--- Hybrid Search (Keyword + Semantic) ---")
	for _, hq := range hybridQueries {
		fmt.Fprintf(stdout, "\nKeyword: %q  Semantic: %q\n", hq.keyword, hq.text)
		qvec, err := embedder.Embed(ctx, hq.text)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL: embed hybrid query: %v\n", err)
			osExit(1)
			return
		}
		results, err := store.HybridSearch(ctx, hq.keyword, qvec, 3)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL: hybrid search: %v\n", err)
			osExit(1)
			return
		}
		for i, r := range results {
			fmt.Fprintf(stdout, "  #%d [score=%.6f] %s\n", i+1, r.Score, r.Content)
		}
	}

	fmt.Fprintln(stdout, "\n=== Manual Test Complete ===")
}
