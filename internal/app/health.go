package app

import (
	"context"
	"os"
	"time"
)

// Health reports the liveness of the application's core dependencies.
type Health struct {
	ProcessAlive     bool `json:"process_alive"`
	SQLite           bool `json:"sqlite"`
	DataDirWritable  bool `json:"data_dir_writable"`
	MediaDirWritable bool `json:"media_dir_writable"`
}

// Health checks process liveness, the SQLite connection, and that the data and
// media directories are writable.
func (a *App) Health(ctx context.Context) Health {
	h := Health{ProcessAlive: true}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	h.SQLite = a.db.PingContext(pingCtx) == nil

	h.DataDirWritable = isWritable(a.cfg.DataDir)
	h.MediaDirWritable = isWritable(a.cfg.UploadsDir)

	return h
}

// isWritable reports whether dir exists and a temporary file can be created
// and removed within it.
func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".health-*")
	if err != nil {
		return false
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return false
	}
	_ = os.Remove(name)
	return true
}
