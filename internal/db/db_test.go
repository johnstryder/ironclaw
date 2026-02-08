package db

import (
	"database/sql"
	"testing"
)

// =============================================================================
// Connect tests
// =============================================================================

func TestConnect_WhenValidFileURL_ShouldReturnDB(t *testing.T) {
	// Given: a valid in-memory libsql URL
	dbURL := "file:test.db?mode=memory&cache=shared"

	// When: connecting
	conn, err := Connect(dbURL)

	// Then: should succeed and return a usable *sql.DB
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer conn.Close()

	if conn == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
}

func TestConnect_WhenValidFileURL_ShouldSatisfySQLDBInterface(t *testing.T) {
	// Given: a valid in-memory libsql URL
	dbURL := "file:test.db?mode=memory&cache=shared"

	// When: connecting
	conn, err := Connect(dbURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer conn.Close()

	// Then: the returned value should satisfy *sql.DB interface
	var _ *sql.DB = conn
}

func TestConnect_WhenValidURL_ShouldBeAbleToPing(t *testing.T) {
	// Given: a valid in-memory libsql URL
	dbURL := "file:test.db?mode=memory&cache=shared"

	// When: connecting
	conn, err := Connect(dbURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer conn.Close()

	// Then: ping should succeed (already verified in Connect, but test explicitly)
	if pingErr := conn.Ping(); pingErr != nil {
		t.Fatalf("expected successful ping, got: %v", pingErr)
	}
}

func TestConnect_WhenInvalidURL_ShouldReturnError(t *testing.T) {
	// Given: a file URL pointing to an impossible path (directory, not a file)
	dbURL := "file:/dev/null/impossible.db"

	// When: connecting
	conn, err := Connect(dbURL)

	// Then: should return an error
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error for invalid file URL, got nil")
	}
}

func TestConnect_WhenValidURL_ShouldExecuteCreateTable(t *testing.T) {
	// Given: a connected in-memory database
	dbURL := "file:test_create.db?mode=memory&cache=shared"
	conn, err := Connect(dbURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer conn.Close()

	// When: executing a CREATE TABLE statement
	_, execErr := conn.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)")

	// Then: should succeed without error
	if execErr != nil {
		t.Fatalf("expected CREATE TABLE to succeed, got: %v", execErr)
	}
}

func TestConnect_WhenValidURL_ShouldInsertAndQuery(t *testing.T) {
	// Given: a connected database with a table
	dbURL := "file:test_crud.db?mode=memory&cache=shared"
	conn, err := Connect(dbURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer conn.Close()

	_, err = conn.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	// When: inserting and querying a row
	_, err = conn.Exec("INSERT INTO users (name) VALUES (?)", "Alice")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var name string
	err = conn.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)

	// Then: should return the inserted data
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if name != "Alice" {
		t.Errorf("want name 'Alice', got %q", name)
	}
}

func TestConnect_WhenValidURL_ShouldSupportForeignKeys(t *testing.T) {
	// Given: a connected database
	dbURL := "file:test_fk.db?mode=memory&cache=shared"
	conn, err := Connect(dbURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer conn.Close()

	// Enable foreign keys
	_, err = conn.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("PRAGMA foreign_keys failed: %v", err)
	}

	// When: creating tables with foreign key constraints
	_, err = conn.Exec("CREATE TABLE parent (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatalf("create parent table failed: %v", err)
	}

	_, err = conn.Exec("CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id))")
	if err != nil {
		t.Fatalf("create child table failed: %v", err)
	}

	// Then: inserting a child with non-existent parent should fail
	_, err = conn.Exec("INSERT INTO child (parent_id) VALUES (999)")
	if err == nil {
		t.Fatal("expected foreign key violation error, got nil")
	}
}

func TestConnect_WhenEmptyURL_ShouldReturnError(t *testing.T) {
	// Given: an empty URL
	dbURL := ""

	// When: connecting
	conn, err := Connect(dbURL)

	// Then: should return an error
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error for empty URL, got nil")
	}
}

func TestConnect_WhenDriverUnknown_ShouldReturnOpenError(t *testing.T) {
	// Given: a broken driver name
	old := driverName
	driverName = "nonexistent_driver"
	defer func() { driverName = old }()

	// When: connecting
	conn, err := Connect("file:test.db?mode=memory&cache=shared")

	// Then: should return an error from sql.Open
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error for unknown driver, got nil")
	}
	if !contains(err.Error(), "failed to open libsql") {
		t.Errorf("error should mention 'failed to open libsql', got: %v", err)
	}
}

// contains is a simple string-contains helper for test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
