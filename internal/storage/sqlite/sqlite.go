// Package sqlite provides SQLite database access for the application.
package sqlite

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Open opens (creating if necessary) the SQLite database at path and
// configures it for safe concurrent use.
//
// The connection pool is intentionally limited to a single open connection:
// SQLite allows only one writer at a time and this keeps locking behaviour
// predictable for the modular monolith.
func Open(path string) (*sql.DB, error) {
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return db, nil
}

// buildDSN constructs a modernc.org/sqlite DSN with the required PRAGMAs.
func buildDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")

	u := url.URL{Scheme: "file", Opaque: filepath.ToSlash(path)}
	u.RawQuery = q.Encode()
	return u.String()
}
