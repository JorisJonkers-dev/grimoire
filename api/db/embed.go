// Package db embeds the SQL migrations so the binary can migrate its own database.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
