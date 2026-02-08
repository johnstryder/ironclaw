package main

import (
	"context"
	"database/sql"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"

	idb "ironclaw/internal/db"
	wa "ironclaw/internal/whatsapp"
)

func init() {
	newWAClientFn = createWhatsmeowClient
}

// connectFn opens a libSQL connection. Replaceable in tests.
var connectFn = idb.Connect

// execPragmaFn enables foreign keys on the database. Replaceable in tests.
var execPragmaFn = func(db *sql.DB) error {
	_, err := db.Exec("PRAGMA foreign_keys = ON")
	return err
}

// newContainerFn wraps sqlstore.NewWithDB + Upgrade. Replaceable in tests.
var newContainerFn = func(db *sql.DB) (*sqlstore.Container, error) {
	container := sqlstore.NewWithDB(db, "sqlite3", nil)
	if err := container.Upgrade(context.Background()); err != nil {
		return nil, err
	}
	return container, nil
}

func createWhatsmeowClient(dbPath string) (wa.WAClient, error) {
	// Open the database via libSQL (pure Go, no CGO required).
	db, err := connectFn(fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	// Enable foreign keys (required by whatsmeow).
	if err := execPragmaFn(db); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Wrap in whatsmeow's sqlstore using sqlite3 dialect (libSQL is wire-compatible).
	container, err := newContainerFn(db)
	if err != nil {
		return nil, fmt.Errorf("sqlstore upgrade: %w", err)
	}

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("device store: %w", err)
	}

	client := whatsmeow.NewClient(device, nil)
	return wa.NewWhatsmeowClient(client), nil
}
