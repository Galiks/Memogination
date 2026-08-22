package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/memomarium/memomarium/migrations"
	"github.com/pressly/goose/v3"
)

// Migrate applies any pending goose migrations to db.
//
// Before running migrations, if the database file exists and migrations are
// pending, a backup copy is written to backupsDir and the most recent
// backupsDir entries are pruned to keep at most keepBackups copies.
func Migrate(db *sql.DB, dbPath, backupsDir string) error {
	if err := backupIfNeeded(db, dbPath, backupsDir); err != nil {
		return err
	}

	goose.SetBaseFS(migrations.Migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

const keepBackups = 3

// backupIfNeeded creates a timestamped backup of the database file when
// migrations are pending and the file already exists.
func backupIfNeeded(db *sql.DB, dbPath, backupsDir string) error {
	pending, err := migrationsPending(db)
	if err != nil {
		return err
	}
	if !pending {
		return nil
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat database file: %w", err)
	}

	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return fmt.Errorf("create backups dir: %w", err)
	}

	name := fmt.Sprintf("memomarium-%s.db", time.Now().Format("2006-01-02-150405"))
	dst := filepath.Join(backupsDir, name)

	data, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("read database for backup: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write database backup: %w", err)
	}

	return pruneBackups(backupsDir, keepBackups)
}

// migrationsPending reports whether any migration has not yet been applied.
func migrationsPending(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'`,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check goose version table: %w", err)
	}
	if n == 0 {
		// No version table yet: pending if any migration files exist.
		max, err := maxMigrationVersion()
		if err != nil {
			return false, err
		}
		return max > 0, nil
	}

	current, err := goose.GetDBVersion(db)
	if err != nil {
		return false, fmt.Errorf("get current migration version: %w", err)
	}
	max, err := maxMigrationVersion()
	if err != nil {
		return false, err
	}
	return current < max, nil
}

// maxMigrationVersion returns the highest version number among embedded
// migration files.
func maxMigrationVersion() (int64, error) {
	entries, err := migrations.Migrations.ReadDir(".")
	if err != nil {
		return 0, fmt.Errorf("read embedded migrations: %w", err)
	}
	var max int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, err := goose.NumericComponent(e.Name())
		if err != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	return max, nil
}

// pruneBackups removes all but the keep most recent backup files.
func pruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read backups dir: %w", err)
	}

	var backups []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "memomarium-") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		backups = append(backups, e.Name())
	}

	// Names sort lexicographically, which matches chronological order.
	sort.Strings(backups)

	for i := 0; i < len(backups)-keep; i++ {
		if err := os.Remove(filepath.Join(dir, backups[i])); err != nil {
			return fmt.Errorf("remove old backup %q: %w", backups[i], err)
		}
	}
	return nil
}
