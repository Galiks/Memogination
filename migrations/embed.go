// Package migrations embeds the SQL migration files used by goose.
package migrations

import "embed"

// Migrations contains all *.sql migration files in this directory.
//
//go:embed *.sql
var Migrations embed.FS
