// Package schema embeds the SQLite DDL shared by client (sync-db) and
// server (agent-telemetry-server). Keeping it dep-free means the server
// binary does not pull in transcript parsing or sessionindex code.
package schema

import _ "embed"

//go:generate go run ./genhash

//go:embed schema.sql
var SQL string
