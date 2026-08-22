// Package sqlite implements the repository interface over modernc.org/sqlite.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/memomarium/memomarium/internal/repository"
)

// timeFmt is the ISO8601 UTC timestamp format used for all TEXT timestamps.
const timeFmt = "2006-01-02T15:04:05.999999999Z07:00"

// dbtx is the minimal interface shared by *sql.DB and *sql.Tx.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Repo implements repository.Repository over a *sql.DB.
type Repo struct {
	queries
}

// New creates a repository backed by db.
func New(db *sql.DB) *Repo {
	return &Repo{queries: queries{q: db}}
}

// WithTx runs fn within a transaction. If fn returns an error the transaction
// is rolled back; otherwise it is committed.
func (r *Repo) WithTx(ctx context.Context, fn func(tx repository.Tx) error) error {
	tx, err := r.q.(*sql.DB).BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(&txRepo{queries: queries{q: tx}}); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// txRepo implements repository.Tx over a *sql.Tx.
type txRepo struct {
	queries
}

// queries holds the shared query logic for both DB and Tx scopes.
type queries struct {
	q dbtx
}

// newID returns a new UUID string.
func newID() string {
	return uuid.NewString()
}

// boolInt converts a bool to a SQLite integer.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableString returns nil for an empty string, otherwise the string.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableTime returns nil for a nil time, otherwise the formatted timestamp.
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(timeFmt)
}

// parseTime parses an ISO8601 timestamp string into a UTC time.Time.
func parseTime(s string) time.Time {
	t, err := time.Parse(timeFmt, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// parseNullableTime converts a nullable timestamp column into a *time.Time.
func parseNullableTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}
