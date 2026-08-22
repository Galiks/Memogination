// Package config loads and resolves application configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds resolved runtime paths and tunables for the application.
type Config struct {
	// DataDir is the root directory for all runtime data.
	DataDir string
	// DBPath is the full path to the SQLite database file.
	DBPath string
	// UploadsDir is the directory where uploaded media is stored.
	UploadsDir string
	// BackupsDir is the directory where pre-migration DB backups are kept.
	BackupsDir string
	// LogsDir is the directory where application logs are written.
	LogsDir string
	// HTTPAddr is the listen address for the HTTP server.
	HTTPAddr string
	// MaxUploadBytes is the maximum allowed upload size in bytes.
	MaxUploadBytes int64
}

// Load resolves the data directory and derives all dependent paths.
//
// If dataDir is empty the default data directory is used
// (%LOCALAPPDATA%\Memomarium on Windows, ~/.memomarium otherwise).
// Passing a non-empty dataDir enables portable mode (e.g. ./data).
func Load(dataDir string) (Config, error) {
	if dataDir == "" {
		dataDir = defaultDataDir()
	}

	cfg := Config{
		DataDir:        dataDir,
		DBPath:         filepath.Join(dataDir, "data", "memomarium.db"),
		UploadsDir:     filepath.Join(dataDir, "uploads", "memes"),
		BackupsDir:     filepath.Join(dataDir, "backups"),
		LogsDir:        filepath.Join(dataDir, "logs"),
		HTTPAddr:       ":8080",
		MaxUploadBytes: 20 * 1024 * 1024, // 20 MB
	}

	if err := cfg.EnsureDirs(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// EnsureDirs creates all runtime subdirectories required by the application.
func (c Config) EnsureDirs() error {
	dirs := []string{
		c.DataDir,
		filepath.Dir(c.DBPath), // data/
		c.UploadsDir,           // uploads/memes/
		c.BackupsDir,           // backups/
		c.LogsDir,              // logs/
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	return nil
}

// defaultDataDir returns the platform-appropriate default data directory.
func defaultDataDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Memomarium")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "data"
	}
	return filepath.Join(home, ".memomarium")
}
