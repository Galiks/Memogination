package http

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/memomarium/memomarium/web"
)

// staticHandler serves the embedded frontend with SPA fallback: any path that
// does not resolve to a file falls back to index.html.
//
// The frontend is embedded via the root-level web package (web/dist). A
// placeholder index.html is committed so the server always builds and runs even
// before the frontend is built; a real build overwrites those files.
func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		// Should not happen because web/dist is embedded, but keep the server
		// runnable with a minimal placeholder if it ever does.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<h1>Memomarium</h1><p>Frontend unavailable.</p>"))
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// SPA fallback: serve index.html for client-side routing.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
