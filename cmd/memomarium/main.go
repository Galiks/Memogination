// Command memomarium is the entry point for the Memomarium game server.
package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/memomarium/memomarium/internal/app"
	"github.com/memomarium/memomarium/internal/config"
	"github.com/memomarium/memomarium/internal/coordinator"
	"github.com/memomarium/memomarium/internal/engine"
	"github.com/memomarium/memomarium/internal/media"
	"github.com/memomarium/memomarium/internal/repository/sqlite"
	"github.com/memomarium/memomarium/internal/session"
	httptransport "github.com/memomarium/memomarium/internal/transport/http"
	"github.com/memomarium/memomarium/internal/transport/websocket"
)

func main() {
	dataDir := flag.String("data-dir", "", "data directory (portable mode: ./data)")
	addr := flag.String("addr", "", "HTTP listen address (overrides config default :8080)")
	flag.Parse()

	cfg, err := config.Load(*dataDir)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	if *addr != "" {
		cfg.HTTPAddr = *addr
	}

	slog.SetDefault(newLogger(cfg.LogsDir))

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("create app", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := application.Close(); err != nil {
			slog.Error("close app", "error", err)
		}
	}()

	// Wire the application service layer.
	repo := sqlite.New(application.DB())
	sessions := session.NewService(repo)
	eng := engine.New()
	hub := websocket.NewHub()
	coord := coordinator.New(repo, eng, sessions, hub)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Recover any rooms whose phase deadline passed while the server was down.
	if err := coord.Recover(ctx); err != nil {
		slog.Error("recover rooms", "error", err)
		os.Exit(1)
	}

	// Run the live-game timeout scheduler so phase deadlines advance during a
	// game, not just at startup. It stops when ctx is cancelled on shutdown.
	coord.StartTimeoutScheduler(ctx, time.Second)

	mediaSvc := &media.Service{UploadsDir: cfg.UploadsDir, MaxBytes: cfg.MaxUploadBytes}
	server := httptransport.New(coord, sessions, mediaSvc, cfg, slog.Default(), hub, application)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start the HTTP server.
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		slog.Error("listen", "addr", cfg.HTTPAddr, "error", err)
		os.Exit(1)
	}
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", "error", err)
			stop()
		}
	}()

	port := portOf(cfg.HTTPAddr)
	slog.Info("memomarium started",
		"http_addr", cfg.HTTPAddr,
		"data_dir", cfg.DataDir,
		"db_path", cfg.DBPath,
		"uploads_dir", cfg.UploadsDir,
	)

	// Log reachable LAN addresses.
	for _, addr := range httptransport.DetectLANAddresses(port) {
		slog.Info("lan address", "url", addr)
	}

	// Best-effort: open the host page in the default browser.
	openBrowser("http://localhost:" + port + "/host")

	<-ctx.Done()
	slog.Info("memomarium shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "error", err)
	}
}

// portOf extracts the port from a listen address like ":8080" or "127.0.0.1:8080".
func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "8080"
	}
	return port
}

// openBrowser opens url in the default browser. It is best-effort and ignores
// errors.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Debug("open browser", "error", err)
	}
}

// newLogger returns a JSON slog logger writing to both stdout and a log file
// in logsDir. If the log file cannot be opened, it falls back to stdout only.
func newLogger(logsDir string) *slog.Logger {
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	f, err := os.OpenFile(
		filepath.Join(logsDir, "memomarium.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	w := io.MultiWriter(os.Stdout, f)
	return slog.New(slog.NewJSONHandler(w, nil))
}
