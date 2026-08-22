// Package app wires configuration and storage into the application root.
package app

import (
	"database/sql"
	"fmt"

	"github.com/memomarium/memomarium/internal/config"
	"github.com/memomarium/memomarium/internal/storage/sqlite"
)

// App is the application root, owning configuration and the database handle.
type App struct {
	cfg config.Config
	db  *sql.DB
}

// New creates the application: it ensures directories exist, opens the SQLite
// database and applies pending migrations.
func New(cfg config.Config) (*App, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("ensure directories: %w", err)
	}

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	if err := sqlite.Migrate(db, cfg.DBPath, cfg.BackupsDir); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &App{cfg: cfg, db: db}, nil
}

// Close gracefully shuts down the application, closing the database.
func (a *App) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// DB returns the underlying database handle.
func (a *App) DB() *sql.DB { return a.db }

// Config returns the resolved configuration.
func (a *App) Config() config.Config { return a.cfg }
