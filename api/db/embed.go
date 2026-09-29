// Package db embeds the SQL migrations and seed data so the binary can prepare its own database.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS

// Seeds holds the pinned compendium snapshot (SRD content under CC-BY-4.0).
//
//go:embed seeds/*.json.gz
var Seeds embed.FS
