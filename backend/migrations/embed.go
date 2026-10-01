// Package migrations embeds the goose SQL migration files so cmd/server can
// run them on startup without relying on the filesystem layout at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
