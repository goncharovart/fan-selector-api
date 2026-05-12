// Package migrations exposes the project's SQL migration files as an embedded
// filesystem so command binaries can apply them without shipping loose .sql
// files alongside the image. The actual SQL lives in plain .sql neighbors so
// goose can still discover them during local development.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
