package syncdb

import _ "embed"

//go:generate go run ./genhash

//go:embed schema.sql
var schemaSQL string

// SchemaHash returns the SHA-256 of schema.sql at build time. Callers outside
// this package — notably the push pipeline that stamps payloads with a
// schema_hash for server-side compatibility checks — read it through this
// accessor so the underlying constant can stay package-private.
func SchemaHash() string {
	return schemaHash
}
