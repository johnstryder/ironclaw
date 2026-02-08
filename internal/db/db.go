package db

import (
	"database/sql"
	"fmt"

	// Import the libSQL driver — registers "libsql" with database/sql.
	// Handles remote URLs (libsql://, https://, wss://).
	_ "github.com/tursodatabase/libsql-client-go/libsql"

	// Import the pure-Go SQLite driver for local file: URLs.
	// libsql-client-go delegates file: URLs to this driver.
	_ "modernc.org/sqlite"
)

// driverName is the database/sql driver to use. Exported for testing only via
// package-level variable; production always uses "libsql".
var driverName = "libsql"

// Connect opens a libSQL database connection and verifies it with a ping.
//
// Supported URL schemes:
//
//	Local file:  "file:path/to/db.db"
//	Remote Turso: "libsql://[db-name].turso.io?authToken=[token]"
func Connect(dbURL string) (*sql.DB, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("database URL must not be empty")
	}

	db, err := sql.Open(driverName, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open libsql: %w", err)
	}

	// Verify the connection is actually reachable.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
