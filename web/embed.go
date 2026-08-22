// Package web embeds the built frontend so the server can serve it without a
// separate static file deployment.
package web

import "embed"

// Dist contains the built frontend (web/dist). A placeholder index.html is
// committed so the server always builds; a real frontend build overwrites it.
//
//go:embed all:dist
var Dist embed.FS
