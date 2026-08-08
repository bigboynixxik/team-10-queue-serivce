// Package migrations contains embedded SQL files for database schema versioning.
package migrations

import "embed"

// FS holds the embedded SQL migration files.
//
//go:embed *.sql
var FS embed.FS
